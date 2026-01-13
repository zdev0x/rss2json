package rss

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestConvertSuccess(t *testing.T) {
	restore := WithHTTPClient(fakeDoer{body: sampleRSS, status: http.StatusOK})
	defer restore()

	resp, err := Convert(context.Background(), "https://example.com/rss")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	if resp.Feed.Title != "Sample Feed" {
		t.Fatalf("unexpected feed title: %s", resp.Feed.Title)
	}
	if resp.Feed.Description != "<p>Demo</p>" {
		t.Fatalf("feed description not unescaped: %q", resp.Feed.Description)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.Content == "" || !strings.Contains(item.Content, "Hello") {
		t.Fatalf("unexpected item content: %s", item.Content)
	}
	if item.Description != "<p>Desc</p>" {
		t.Fatalf("description should keep html, got %q", item.Description)
	}
	if item.Author != "John Doe" {
		t.Fatalf("unexpected author: %s", item.Author)
	}
}

func TestConvertMissingURL(t *testing.T) {
	if _, err := Convert(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestConvertBadXML(t *testing.T) {
	restore := WithHTTPClient(fakeDoer{body: "<rss><channel><title>bad</title></channel>", status: http.StatusOK})
	defer restore()

	if _, err := Convert(context.Background(), "https://example.com/rss"); err == nil {
		t.Fatal("expected XML parse error")
	}
}

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Sample Feed</title>
    <link>https://example.com</link>
    <description>&lt;p&gt;Demo&lt;/p&gt;</description>
    <managingEditor>Editor Name</managingEditor>
    <image>
      <url>https://example.com/logo.png</url>
    </image>
    <item>
      <title>Hello</title>
      <link>https://example.com/post</link>
      <description><![CDATA[<p>Desc</p>]]></description>
      <dc:creator>John Doe</dc:creator>
      <content:encoded><![CDATA[<p>Hello World</p>]]></content:encoded>
      <pubDate>Mon, 01 Jan 2024 00:00:00 GMT</pubDate>
      <guid>abc123</guid>
    </item>
  </channel>
</rss>`

// newTCP4Server 保证在 IPv4 下监听，避免沙箱禁用 IPv6。
type fakeDoer struct {
	body   string
	status int
}

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(bytes.NewBufferString(f.body)),
	}, nil
}
