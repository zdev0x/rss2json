package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/zdev0x/rss2json/internal/server"
)

func main() {
	addr := resolveListenAddr()
	opts := server.Options{
		APIKey:           strings.TrimSpace(os.Getenv("API_KEY")),
		EnableRequestLog: shouldLogRequest(),
	}
	printBanner(addr, opts)

	if err := http.ListenAndServe(addr, server.NewHandler(opts)); err != nil {
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

// printBanner 输出启动信息，突出 rss2json。
func printBanner(addr string, opts server.Options) {
	border := strings.Repeat("#", 56)
	logStatus := "off"
	if opts.EnableRequestLog {
		logStatus = "on"
	}
	authStatus := "off"
	if strings.TrimSpace(opts.APIKey) != "" {
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
