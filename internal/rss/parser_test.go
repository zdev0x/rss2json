package rss

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zdev0x/rss2json/internal/model"
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
	if resp.Version != model.APIVersion {
		t.Fatalf("unexpected version: %s", resp.Version)
	}
	if resp.Feed == nil {
		t.Fatal("expected feed to be set")
	}
	if resp.Feed.Title != "Sample Feed" {
		t.Fatalf("unexpected feed title: %s", resp.Feed.Title)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.Content == "" || !strings.Contains(item.Content, "Hello") {
		t.Fatalf("unexpected item content: %s", item.Content)
	}
	if item.Description == "" || !strings.Contains(item.Description, "Desc") {
		t.Fatalf("description should keep html, got %q", item.Description)
	}
	if item.GUID != "abc123" {
		t.Fatalf("unexpected guid: %s", item.GUID)
	}
}

func TestConvertMissingURL(t *testing.T) {
	if _, err := Convert(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty url")
	} else if !IsInvalidInput(err) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestConvertBadXML(t *testing.T) {
	restore := WithHTTPClient(fakeDoer{body: "<rss><channel><title>bad</title></channel>", status: http.StatusOK})
	defer restore()

	if _, err := Convert(context.Background(), "https://example.com/rss"); err == nil {
		t.Fatal("expected XML parse error")
	} else if IsInvalidInput(err) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestConvertBodyTooLarge(t *testing.T) {
	t.Setenv(maxFeedBytesEnv, "64")
	restore := WithHTTPClient(fakeDoer{body: sampleRSS, status: http.StatusOK})
	defer restore()

	if _, err := Convert(context.Background(), "https://example.com/rss"); err == nil {
		t.Fatal("expected size limit error")
	} else if IsInvalidInput(err) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestConvertAtomSuccess(t *testing.T) {
	restore := WithHTTPClient(fakeDoer{body: sampleAtom, status: http.StatusOK})
	defer restore()

	resp, err := Convert(context.Background(), "https://example.com/atom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Feed == nil {
		t.Fatal("expected feed to be set")
	}
	if resp.Feed.Title != "Atom Feed" {
		t.Fatalf("unexpected feed title: %s", resp.Feed.Title)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.Title != "Atom Item" {
		t.Fatalf("unexpected item title: %s", item.Title)
	}
	if item.Author == nil || item.Author.Name != "Jane Doe" {
		t.Fatalf("unexpected author: %v", item.Author)
	}
	if got := firstNonEmpty(item.Published, item.Updated); got != "2024-01-02T00:00:00Z" {
		t.Fatalf("unexpected pub date: %s", got)
	}
	if item.Description != "<p>Atom Summary</p>" {
		t.Fatalf("unexpected description: %s", item.Description)
	}
	if item.Content != "<p>Atom Content</p>" {
		t.Fatalf("unexpected content: %s", item.Content)
	}
}

func TestNewHTTPClientFromEnvHTTPProxy(t *testing.T) {
	t.Setenv("RSS_PROXY", "http://127.0.0.1:8888")
	c := newHTTPClientFromEnv()
	client, ok := c.(*http.Client)
	if !ok {
		t.Fatalf("expected *http.Client")
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatalf("expected http transport with proxy")
	}
}

func TestNewHTTPClientFromEnvSocks5(t *testing.T) {
	t.Setenv("RSS_PROXY", "socks5://127.0.0.1:1080")
	c := newHTTPClientFromEnv()
	client, ok := c.(*http.Client)
	if !ok {
		t.Fatalf("expected *http.Client")
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport")
	}
	if tr.DialContext == nil {
		t.Fatalf("expected DialContext to be set for socks5")
	}
}

func TestCustomHeadersFromEnv(t *testing.T) {
	t.Setenv("RSS_HEADERS", "X-Test=ok,User-Agent=custom-agent")
	restore := WithHTTPClient(headerDoer{t: t})
	defer restore()

	if _, err := Convert(context.Background(), "https://example.com/rss"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type headerDoer struct {
	t *testing.T
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (h headerDoer) Do(req *http.Request) (*http.Response, error) {
	h.t.Helper()
	if got := req.Header.Get("X-Test"); got != "ok" {
		h.t.Fatalf("header X-Test not set, got %q", got)
	}
	if ua := req.Header.Get("User-Agent"); ua != "custom-agent" {
		h.t.Fatalf("user-agent not overridden, got %q", ua)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(sampleRSS)),
	}, nil
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

const sampleAtom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <link href="https://example.com"/>
  <updated>2024-01-01T00:00:00Z</updated>
  <author>
    <name>Jane Doe</name>
  </author>
  <id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
  <subtitle>&lt;p&gt;Atom Desc&lt;/p&gt;</subtitle>
  <entry>
    <title>Atom Item</title>
    <link href="https://example.com/atom/1"/>
    <id>tag:example.com,2024:1</id>
    <updated>2024-01-02T00:00:00Z</updated>
    <summary><![CDATA[<p>Atom Summary</p>]]></summary>
    <content type="html">&lt;p&gt;Atom Content&lt;/p&gt;</content>
    <author>
      <name>Jane Doe</name>
    </author>
  </entry>
</feed>`

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
