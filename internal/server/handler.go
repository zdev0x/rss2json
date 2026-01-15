package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/zdev0x/rss2json/internal/model"
	"github.com/zdev0x/rss2json/internal/rss"
)

// serviceStart 记录服务启动时间，用于健康检查输出。
var serviceStart = time.Now()

// ConvertHandler 处理 /api/v1/rss2json 请求。
func ConvertHandler(w http.ResponseWriter, r *http.Request) {
	// 固定使用查询参数 url。
	rssURL := r.URL.Query().Get("url")

	resp, err := rss.Convert(r.Context(), rssURL)
	if err != nil {
		status, message := mapError(err)
		writeJSON(w, status, model.Response{
			Status:  "error",
			Version: model.APIVersion,
			Message: message,
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func mapError(err error) (int, string) {
	if rss.IsInvalidInput(err) {
		// 情况 1: 输入参数缺失（422 是非常好的选择）
		return http.StatusUnprocessableEntity, "Missing rss url."
	}

	if isTimeout(err) {
		// 情况 2: 抓取超时
		// 建议：改用 408 (Request Timeout) 或直接用 400
		// 这样 Cloudflare 会认为这是业务超时，而不是你的服务器宕机
		return http.StatusRequestTimeout, "RSS fetch timeout. The target server responded too slowly."
	}

	// 情况 3: 无法下载、DNS 解析失败、404 等
	// 原代码返回 StatusBadGateway (502)，这是导致 Cloudflare 报错的罪魁祸首
	// 建议：改用 400 Bad Request 或 422
	return http.StatusBadRequest, "Cannot download this RSS feed. Please check if the URL is valid and accessible."
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false) // 保留 HTML 字符，避免被转义为 \u003c 之类的形式。
	_ = enc.Encode(payload)
}

// 健康检查就接口
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	_ = r
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(serviceStart).Seconds(),
	})

}
