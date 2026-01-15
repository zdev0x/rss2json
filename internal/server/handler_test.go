package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zdev0x/rss2json/internal/model"
	"github.com/zdev0x/rss2json/internal/rss"
)

func TestConvertHandlerOK(t *testing.T) {
	restore := rss.WithHTTPClient(fakeDoer{
		status: http.StatusOK,
		body:   `<?xml version="1.0" encoding="UTF-8"?><rss xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel><title>a</title><link>b</link><description>d</description><item><title>x</title><link>y</link><content:encoded><![CDATA[<p>Hello</p>]]></content:encoded></item></channel></rss>`,
	})
	defer restore()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rss2json?url="+url.QueryEscape("https://example.com/rss"), nil)
	rr := httptest.NewRecorder()

	ConvertHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("unexpected status: %v", resp["status"])
	}
	if resp["version"] != model.APIVersion {
		t.Fatalf("unexpected version: %v", resp["version"])
	}
	items, ok := resp["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 top-level item, got %v", resp["items"])
	}
	if feedMap, ok := resp["feed"].(map[string]interface{}); ok {
		if _, exists := feedMap["items"]; exists {
			t.Fatalf("feed should not contain items")
		}
	}

	if strings.Contains(rr.Body.String(), `\u003c`) {
		t.Fatalf("html should not be escaped: %s", rr.Body.String())
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
}

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

type errorDoer struct {
	err error
}

func (e errorDoer) Do(req *http.Request) (*http.Response, error) {
	return nil, e.err
}

type errBoom struct{}

func (errBoom) Error() string {
	return "boom"
}

type timeoutErr struct{}

func (timeoutErr) Error() string {
	return "timeout"
}

func (timeoutErr) Timeout() bool {
	return true
}

func (timeoutErr) Temporary() bool {
	return true
}

func TestConvertHandlerError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rss2json", nil)
	rr := httptest.NewRecorder()

	ConvertHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["status"] != "error" {
		t.Fatalf("unexpected status: %v", resp["status"])
	}
	if resp["version"] != model.APIVersion {
		t.Fatalf("unexpected version: %v", resp["version"])
	}
	if resp["message"] != "Missing rss url." {
		t.Fatalf("unexpected message: %v", resp["message"])
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
}

func TestConvertHandlerUpstreamError(t *testing.T) {
	restore := rss.WithHTTPClient(errorDoer{err: errBoom{}})
	defer restore()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rss2json?url="+url.QueryEscape("https://example.com/rss"), nil)
	rr := httptest.NewRecorder()

	ConvertHandler(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["message"] != "Cannot download this RSS feed, make sure the Rss URL is correct." {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestConvertHandlerTimeout(t *testing.T) {
	restore := rss.WithHTTPClient(errorDoer{err: timeoutErr{}})
	defer restore()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rss2json?url="+url.QueryEscape("https://example.com/rss"), nil)
	rr := httptest.NewRecorder()

	ConvertHandler(rr, req)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rr.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	HealthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("unexpected status: %v", resp["status"])
	}
	if _, ok := resp["uptime"]; !ok {
		t.Fatalf("uptime field missing")
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
}
