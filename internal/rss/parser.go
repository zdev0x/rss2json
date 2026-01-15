package rss

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/zdev0x/rss2json/internal/model"
)

// httpClientTimeout 定义 RSS 拉取超时时间。
const (
	httpClientTimeout   = 10 * time.Second
	dialTimeout         = 5 * time.Second
	tlsHandshakeTimeout = 5 * time.Second
	responseHeaderTime  = 5 * time.Second
	idleConnTimeout     = 30 * time.Second
	maxIdleConns        = 100
	maxIdleConnsPerHost = 10
	defaultMaxFeedBytes = int64(10 << 20) // 10 MiB
)

const maxFeedBytesEnv = "RSS_MAX_BYTES"

type ErrorKind int

const (
	ErrorKindInvalidInput ErrorKind = iota + 1
	ErrorKindUpstream
)

type FeedError struct {
	Kind ErrorKind
	Err  error
}

func (e *FeedError) Error() string {
	return e.Err.Error()
}

func (e *FeedError) Unwrap() error {
	return e.Err
}

func newInvalidInputErr(err error) error {
	return &FeedError{Kind: ErrorKindInvalidInput, Err: err}
}

func newUpstreamErr(err error) error {
	return &FeedError{Kind: ErrorKindUpstream, Err: err}
}

func IsInvalidInput(err error) bool {
	var feedErr *FeedError
	return errors.As(err, &feedErr) && feedErr.Kind == ErrorKindInvalidInput
}

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

// fetchAndParse 从给定 URL 拉取 Feed 并解析为 gofeed 结构。
func fetchAndParse(ctx context.Context, url string) (*gofeed.Feed, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, newInvalidInputErr(fmt.Errorf("创建请求失败: %w", err))
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	applyCustomHeaders(req)

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, nil, newUpstreamErr(fmt.Errorf("下载 RSS 失败: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, newUpstreamErr(fmt.Errorf("RSS 返回非 2xx 状态码: %d", resp.StatusCode))
	}

	reader := io.Reader(resp.Body)
	var limited *io.LimitedReader
	maxBytes := maxFeedBytes()
	if maxBytes > 0 {
		limited = &io.LimitedReader{R: resp.Body, N: maxBytes + 1}
		reader = limited
	}

	var buf bytes.Buffer
	tee := io.TeeReader(reader, &buf)

	parser := gofeed.NewParser()
	feed, err := parser.Parse(tee)
	if err != nil {
		if limited != nil && limited.N == 0 {
			return nil, nil, newUpstreamErr(fmt.Errorf("RSS 内容超过限制: %d bytes", maxBytes))
		}
		return nil, nil, newUpstreamErr(fmt.Errorf("解析 RSS 失败: %w", err))
	}
	if limited != nil && limited.N == 0 {
		return nil, nil, newUpstreamErr(fmt.Errorf("RSS 内容超过限制: %d bytes", maxBytes))
	}
	thumbnails := extractItemThumbnails(buf.Bytes())
	return feed, thumbnails, nil
}

// Convert 将给定 URL 的 RSS 转为统一 JSON 模型。
func Convert(ctx context.Context, url string) (model.Response, error) {
	if url == "" {
		return model.Response{}, newInvalidInputErr(errors.New("缺少 rss url"))
	}

	feed, thumbnails, err := fetchAndParse(ctx, url)
	if err != nil {
		return model.Response{}, err
	}
	stripExtensions(feed)

	items := make([]*model.ItemMeta, 0, len(feed.Items))
	for i, item := range feed.Items {
		thumbnail := ""
		if i < len(thumbnails) {
			thumbnail = thumbnails[i]
		}
		items = append(items, model.NewItemMeta(item, thumbnail))
	}

	return model.Response{
		Status:  "ok",
		Version: model.APIVersion,
		Feed:    model.NewFeedMeta(feed),
		Items:   items,
	}, nil
}

// stripExtensions 移除 Feed 与 Item 的扩展字段，避免对外展示。
func stripExtensions(feed *gofeed.Feed) {
	if feed == nil {
		return
	}
	feed.Extensions = nil
	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		item.Extensions = nil
	}
}

func extractItemThumbnails(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	thumbnails := make([]string, 0)
	inItem := false
	current := ""
	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return thumbnails
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if name == "item" || name == "entry" {
				inItem = true
				current = ""
				continue
			}
			if !inItem || name != "thumbnail" {
				continue
			}
			if current != "" {
				_ = decoder.Skip()
				continue
			}
			if url := attrURL(t.Attr); url != "" {
				current = url
				_ = decoder.Skip()
				continue
			}
			var value string
			if err := decoder.DecodeElement(&value, &t); err == nil {
				current = strings.TrimSpace(value)
			}
		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			if name == "item" || name == "entry" {
				if inItem {
					thumbnails = append(thumbnails, strings.TrimSpace(current))
				}
				inItem = false
			}
		}
	}
	return thumbnails
}

func attrURL(attrs []xml.Attr) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name.Local, "url") {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func maxFeedBytes() int64 {
	raw := strings.TrimSpace(os.Getenv(maxFeedBytesEnv))
	if raw == "" {
		return defaultMaxFeedBytes
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || val <= 0 {
		return defaultMaxFeedBytes
	}
	return val
}

// newHTTPClientFromEnv 构造支持代理的 http.Client。
func newHTTPClientFromEnv() httpDoer {
	proxyEnv := strings.TrimSpace(os.Getenv("RSS_PROXY"))

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTime,
		IdleConnTimeout:       idleConnTimeout,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		ExpectContinueTimeout: time.Second,
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
	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}
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
