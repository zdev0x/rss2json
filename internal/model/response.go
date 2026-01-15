package model

import (
	"bytes"
	"encoding/json"
	"strings"

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
	if image, ok := payload["image"].(map[string]interface{}); ok {
		if url, ok := image["url"].(string); ok {
			payload["image"] = url
		} else {
			payload["image"] = ""
		}
	}
	return marshalJSONNoEscape(payload)
}

// ItemMeta 表示对外保留字段的 Item 结构。
type ItemMeta struct {
	*Item
	Thumbnail string
}

// NewItemMeta 构造 ItemMeta。
func NewItemMeta(item *Item, thumbnail string) *ItemMeta {
	if item == nil {
		return nil
	}
	return &ItemMeta{Item: item, Thumbnail: thumbnail}
}

// MarshalJSON 将 author 扁平化为字符串。
func (i ItemMeta) MarshalJSON() ([]byte, error) {
	if i.Item == nil {
		return []byte("null"), nil
	}
	raw, err := json.Marshal(i.Item)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if author, ok := payload["author"]; ok {
		if authorMap, ok := author.(map[string]interface{}); ok {
			if name, ok := authorMap["name"].(string); ok {
				payload["author"] = name
			} else {
				payload["author"] = ""
			}
		}
	}
	delete(payload, "publishedParsed")
	delete(payload, "updatedParsed")
	if strings.TrimSpace(i.Thumbnail) != "" {
		payload["thumbnail"] = i.Thumbnail
	}
	return marshalJSONNoEscape(payload)
}

func marshalJSONNoEscape(payload interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return data, nil
}

// Response 表示 API 的统一返回结构。
type Response struct {
	Status  string      `json:"status"`
	Version string      `json:"version"`
	Feed    *FeedMeta   `json:"feed,omitempty"`
	Items   []*ItemMeta `json:"items,omitempty"`
	Message string      `json:"message,omitempty"`
}
