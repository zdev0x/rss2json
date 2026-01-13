package rss

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/zdev0x/rss2json/internal/model"
)

// httpClientTimeout 定义 RSS 拉取超时时间。
const httpClientTimeout = 10 * time.Second

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var defaultHTTPClient httpDoer = &http.Client{Timeout: httpClientTimeout}

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
	req.Header.Set("User-Agent", "rss2json/1.0")

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
