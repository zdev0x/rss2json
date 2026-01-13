package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zdev0x/rss2json/internal/server"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/rss2json", server.ConvertHandler)
	mux.HandleFunc("/health", server.HealthHandler)

	addr := resolveListenAddr()
	printBanner(addr)

	var handler http.Handler = mux
	if shouldLogRequest() {
		handler = withRequestLog(mux)
	}

	if key := strings.TrimSpace(os.Getenv("API_KEY")); key != "" {
		handler = withAPIKeyAuth(handler, key)
	}

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// resolveListenAddr 支持通过环境变量配置监听地址，便于容器暴露端口。
func resolveListenAddr() string {
	if addr := strings.TrimSpace(os.Getenv("LISTEN_ADDR")); addr != "" {
		return addr
	}

	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			return "0.0.0.0" + port
		}
		return "0.0.0.0:" + port
	}

	return "0.0.0.0:8080"
}

// withAPIKeyAuth 启用基于 Authorization: Bearer <API_KEY> 的简单鉴权。
func withAPIKeyAuth(next http.Handler, key string) http.Handler {
	token := strings.TrimSpace(key)
	expected := "bearer " + strings.ToLower(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.ToLower(strings.TrimSpace(r.Header.Get("Authorization")))
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status":"error","message":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// printBanner 输出启动信息，突出 rss2json。
func printBanner(addr string) {
	border := strings.Repeat("#", 56)
	logStatus := "off"
	if shouldLogRequest() {
		logStatus = "on"
	}
	authStatus := "off"
	if strings.TrimSpace(os.Getenv("API_KEY")) != "" {
		authStatus = "on"
	}
	hostForURL := addr
	if strings.HasPrefix(hostForURL, ":") {
		hostForURL = "127.0.0.1" + hostForURL
	}
	httpBase := "http://" + hostForURL
	logo := []string{
		"   ____  ____  ____  ____   ___   ___   _   _ ",
		"  |  _ \\|  _ \\| ___||___ \\ / _ \\ / _ \\ | \\ | |",
		"  | |_) | |_) |___ \\  __) | | | | | | ||  \\| |",
		"  |  _ <|  __/ ___) |/ __/| |_| | |_| || |\\  |",
		"  |_| \\_\\_|   |____/|_____|\\___/ \\___(_)_| \\_|",
	}

	log.Printf("\n%s\n%s\n  Listen: %s\n  API:    %s/api/v1/rss2json?url=<rss_url>\n  Log:    %s (REQUEST_LOG)\n  Auth:   %s (API_KEY)\n%s", border, strings.Join(logo, "\n"), addr, httpBase, logStatus, authStatus, border)
}

// shouldLogRequest 通过环境变量控制请求日志开关，默认关闭。
func shouldLogRequest() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("REQUEST_LOG")))
	return val == "1" || val == "true" || val == "on"
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
