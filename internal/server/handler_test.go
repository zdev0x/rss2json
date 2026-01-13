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

	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content-type: %s", ct)
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
