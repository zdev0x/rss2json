package model

import (
	"encoding/json"

	"github.com/mmcdole/gofeed"
)

// APIVersion 定义对外响应结构版本。
const APIVersion = "v1"

// Feed 表示 RSS/Atom 源的原始结构，直接使用 gofeed.Feed。
type Feed = gofeed.Feed

// Item 表示 RSS/Atom 文章的原始结构，直接使用 gofeed.Item。
type Item = gofeed.Item

// FeedMeta 表示去除 items 的 Feed 结构，用于顶层 items 输出。
type FeedMeta struct {
	*Feed
}

// NewFeedMeta 构造 FeedMeta。
func NewFeedMeta(feed *Feed) *FeedMeta {
	if feed == nil {
		return nil
	}
	return &FeedMeta{Feed: feed}
}

// MarshalJSON 移除 items 字段，避免与顶层 items 重复。
func (f FeedMeta) MarshalJSON() ([]byte, error) {
	if f.Feed == nil {
		return []byte("null"), nil
	}
	raw, err := json.Marshal(f.Feed)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	delete(payload, "items")
	return json.Marshal(payload)
}

// Response 表示 API 的统一返回结构。
type Response struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Feed    *FeedMeta `json:"feed,omitempty"`
	Items   []*Item   `json:"items,omitempty"`
	Message string    `json:"message,omitempty"`
}
