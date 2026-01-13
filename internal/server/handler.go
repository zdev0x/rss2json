package server

import (
	"encoding/json"
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
		writeJSON(w, http.StatusBadRequest, model.Response{
			Status:  "error",
			Message: "Cannot download this RSS feed, make sure the Rss URL is correct.",
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
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
