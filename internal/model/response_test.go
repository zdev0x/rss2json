package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestItemMetaMarshalJSONFlattensAuthor(t *testing.T) {
	meta := ItemMeta{
		Item: &gofeed.Item{
			Title:  "Hello",
			Author: &gofeed.Person{Name: "张三"},
		},
	}

	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if payload["author"] != "张三" {
		t.Fatalf("expected author to be flattened, got %v", payload["author"])
	}
}

func TestItemMetaMarshalJSONAddsThumbnail(t *testing.T) {
	meta := ItemMeta{
		Item: &gofeed.Item{
			Title: "Hello",
		},
		Thumbnail: "https://example.com/thumb.jpg",
	}

	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if payload["thumbnail"] != "https://example.com/thumb.jpg" {
		t.Fatalf("expected thumbnail to be set, got %v", payload["thumbnail"])
	}
}

func TestItemMetaMarshalJSONDropsParsedTimes(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	meta := ItemMeta{
		Item: &gofeed.Item{
			Title:           "Hello",
			PublishedParsed: &now,
			UpdatedParsed:   &now,
		},
	}

	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := payload["publishedParsed"]; ok {
		t.Fatalf("publishedParsed should be removed")
	}
	if _, ok := payload["updatedParsed"]; ok {
		t.Fatalf("updatedParsed should be removed")
	}
}
