package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/zdev0x/rss2json/internal/rss"
)

func TestMapErrorInvalidInput(t *testing.T) {
	_, err := rss.Convert(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty url")
	}

	status, _ := mapError(err)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", status)
	}
}

func TestMapErrorTimeout(t *testing.T) {
	status, _ := mapError(context.DeadlineExceeded)
	if status != http.StatusRequestTimeout {
		t.Fatalf("expected status 408, got %d", status)
	}
}

func TestMapErrorUpstream(t *testing.T) {
	status, _ := mapError(errors.New("upstream error"))
	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", status)
	}
}
