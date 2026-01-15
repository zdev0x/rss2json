package server

import (
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zdev0x/rss2json/internal/model"
)

// Options 定义 HTTP 服务相关选项。
type Options struct {
	APIKey           string
	EnableRequestLog bool
}

// NewHandler 构造带路由与中间件的 HTTP Handler。
func NewHandler(opts Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/rss2json", ConvertHandler)
	mux.HandleFunc("/health", HealthHandler)

	var handler http.Handler = mux
	if opts.EnableRequestLog {
		handler = withRequestLog(handler)
	}
	if key := strings.TrimSpace(opts.APIKey); key != "" {
		handler = withAPIKeyAuth(handler, key)
	}

	return handler
}

// withAPIKeyAuth 启用基于 Authorization: Bearer <API_KEY> 的简单鉴权。
func withAPIKeyAuth(next http.Handler, key string) http.Handler {
	token := strings.TrimSpace(key)
	expected := []byte("bearer " + strings.ToLower(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.ToLower(strings.TrimSpace(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare([]byte(auth), expected) != 1 {
			writeJSON(w, http.StatusUnauthorized, model.Response{
				Status:  "error",
				Version: model.APIVersion,
				Message: "unauthorized",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRequestLog 为 handler 增加最小访问日志，记录方法、路径、状态码与耗时。
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("[request] %s %s %d %s ip=%s", r.Method, r.URL.RequestURI(), rec.status, time.Since(start), clientIP(r))
	})
}

// statusRecorder 记录响应状态码。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(statusCode int) {
	s.status = statusCode
	s.ResponseWriter.WriteHeader(statusCode)
}

// clientIP 提取请求端 IP，优先使用 X-Forwarded-For。
func clientIP(r *http.Request) string {
	xff := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
