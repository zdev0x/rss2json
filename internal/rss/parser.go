package rss

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zdev0x/rss2json/internal/model"
)

// httpClientTimeout 定义 RSS 拉取超时时间。
const httpClientTimeout = 10 * time.Second

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// defaultHTTPClient 默认使用环境变量配置的 HTTP 客户端，支持 HTTP/HTTPS/SOCKS5 代理。
var defaultHTTPClient httpDoer = newHTTPClientFromEnv()

// WithHTTPClient 在测试场景中替换默认 HTTP 客户端，返回恢复函数。
func WithHTTPClient(d httpDoer) func() {
	prev := defaultHTTPClient
	defaultHTTPClient = d
	return func() {
		defaultHTTPClient = prev
	}
}

// fetchAndDecode 从给定 URL 拉取 RSS 并解析为内部结构。
func fetchAndDecode(ctx context.Context, url string) (*rssRoot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	applyCustomHeaders(req)

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载 RSS 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("RSS 返回非 2xx 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 RSS 内容失败: %w", err)
	}

	var root rssRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("解析 RSS 失败: %w", err)
	}
	return &root, nil
}

// Convert 将给定 URL 的 RSS 转为统一 JSON 模型。
func Convert(ctx context.Context, url string) (model.Response, error) {
	if url == "" {
		return model.Response{}, errors.New("缺少 rss url")
	}

	root, err := fetchAndDecode(ctx, url)
	if err != nil {
		return model.Response{}, err
	}

	channel := root.Channel
	feed := model.Feed{
		URL:         url,
		Title:       channel.Title,
		Link:        channel.Link,
		Author:      channel.ManagingEditor,
		Description: html.UnescapeString(channel.Description),
		Image:       channel.Image.URL,
	}

	items := make([]model.Item, 0, len(channel.Items))
	for _, it := range channel.Items {
		items = append(items, model.Item{
			Title:       it.Title,
			Link:        it.Link,
			Author:      firstNonEmpty(it.Author, it.Creator),
			Description: html.UnescapeString(it.Description),
			Content:     html.UnescapeString(it.Content),
			PubDate:     it.PubDate,
			Guid:        it.GUID.Value,
		})
	}

	return model.Response{
		Status: "ok",
		Feed:   feed,
		Items:  items,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// newHTTPClientFromEnv 构造支持代理的 http.Client。
func newHTTPClientFromEnv() httpDoer {
	proxyEnv := strings.TrimSpace(os.Getenv("RSS_PROXY"))

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	if proxyEnv == "" {
		return &http.Client{Timeout: httpClientTimeout, Transport: tr}
	}

	u, err := url.Parse(proxyEnv)
	if err != nil {
		return &http.Client{Timeout: httpClientTimeout, Transport: tr}
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		tr.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		proxyAddr := u.Host
		if u.Port() == "" {
			proxyAddr = net.JoinHostPort(u.Hostname(), "1080")
		}
		tr.Proxy = nil
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSocks5(ctx, proxyAddr, addr)
		}
	default:
		// 未知 scheme 时退回默认设置，避免启动失败。
	}

	return &http.Client{Timeout: httpClientTimeout, Transport: tr}
}

// applyCustomHeaders 从环境变量解析自定义头并设置到请求上。
// 格式：RSS_HEADERS="Key=Value,Another=Value2"；若包含 User-Agent 将覆盖默认值。
func applyCustomHeaders(req *http.Request) {
	hdrs := customHeadersFromEnv()
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
}

func customHeadersFromEnv() map[string]string {
	raw := strings.TrimSpace(os.Getenv("RSS_HEADERS"))
	if raw == "" {
		return nil
	}
	res := make(map[string]string)
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key == "" {
			continue
		}
		res[key] = val
	}
	return res
}

// dialSocks5 建立 SOCKS5 连接，仅支持无认证模式。
func dialSocks5(ctx context.Context, proxyAddr string, targetAddr string) (net.Conn, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("连接 SOCKS5 代理失败: %w", err)
	}

	// 方法协商：版本 5，1 种方法，无认证(0x00)。
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 方法协商失败: %w", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 方法响应失败: %w", err)
	}
	if reply[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 不支持的认证方法: 0x%x", reply[1])
	}

	// CONNECT 请求
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("目标地址不合法: %w", err)
	}
	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum < 1 || portNum > 65535 {
		conn.Close()
		return nil, fmt.Errorf("目标端口不合法: %s", portStr)
	}

	var addrBuf []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			addrBuf = append([]byte{0x01}, ip4...)
		} else {
			addrBuf = append([]byte{0x04}, ip.To16()...)
		}
	} else {
		hostBytes := []byte(host)
		if len(hostBytes) > 255 {
			conn.Close()
			return nil, fmt.Errorf("域名过长")
		}
		addrBuf = append([]byte{0x03, byte(len(hostBytes))}, hostBytes...)
	}

	req := []byte{0x05, 0x01, 0x00}
	req = append(req, addrBuf...)
	req = append(req, byte(portNum>>8), byte(portNum))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 CONNECT 发送失败: %w", err)
	}

	// 响应：VER REP RSV ATYP BND.ADDR BND.PORT
	resp := make([]byte, 4)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 CONNECT 响应失败: %w", err)
	}
	if resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 CONNECT 被拒绝: 0x%x", resp[1])
	}

	var skip int
	switch resp[3] {
	case 0x01:
		skip = 4
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 读取域名长度失败: %w", err)
		}
		skip = int(lenBuf[0])
	case 0x04:
		skip = 16
	default:
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 未知地址类型: 0x%x", resp[3])
	}
	if skip > 0 {
		buf := make([]byte, skip+2) // 包含端口 2 字节
		if _, err := io.ReadFull(conn, buf); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 读取绑定地址失败: %w", err)
		}
	}

	return conn, nil
}

// 以下为 XML 映射结构，尽量覆盖常见 RSS 字段。

type rssRoot struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title          string    `xml:"title"`
	Link           string    `xml:"link"`
	Description    string    `xml:"description"`
	ManagingEditor string    `xml:"managingEditor"`
	Image          rssImage  `xml:"image"`
	Items          []rssItem `xml:"item"`
}

type rssImage struct {
	URL string `xml:"url"`
}

type rssGuid struct {
	Value string `xml:",chardata"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	Author      string  `xml:"author"`
	Creator     string  `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Content     string  `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate     string  `xml:"pubDate"`
	GUID        rssGuid `xml:"guid"`
}
