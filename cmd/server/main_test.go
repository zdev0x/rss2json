package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zdev0x/rss2json/internal/server"
)

func TestWithAPIKeyAuthSuccess(t *testing.T) {
	handler := server.NewHandler(server.Options{APIKey: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWithAPIKeyAuthFail(t *testing.T) {
	handler := server.NewHandler(server.Options{APIKey: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
