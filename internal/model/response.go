package model

// Feed 表示 RSS 源的元信息。
type Feed struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Link        string `json:"link,omitempty"`
	Author      string `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
}

// Item 表示单条 RSS 文章。
type Item struct {
	Title       string `json:"title,omitempty"`
	Link        string `json:"link,omitempty"`
	Author      string `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	PubDate     string `json:"pubDate,omitempty"`
	Guid        string `json:"guid,omitempty"`
}

// Response 表示 API 的统一返回结构。
type Response struct {
	Status  string `json:"status"`
	Feed    Feed   `json:"feed,omitempty"`
	Items   []Item `json:"items,omitempty"`
	Message string `json:"message,omitempty"`
}
