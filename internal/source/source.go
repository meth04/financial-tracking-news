package source

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

type FetchedItem struct {
	SourceKey    string         `json:"source_key"`
	SourceID     uuid.UUID      `json:"source_id"`
	RawURL       string         `json:"raw_url"`
	CanonicalURL string         `json:"canonical_url"`
	Title        string         `json:"title"`
	Excerpt      string         `json:"excerpt"`
	ContentHTML  string         `json:"content_html"`
	ContentText  string         `json:"content_text"`
	Author       string         `json:"author"`
	PublishedAt  *time.Time     `json:"published_at,omitempty"`
	FetchedAt    time.Time      `json:"fetched_at"`
	RawPayload   []byte         `json:"-"`
	ContentType  string         `json:"content_type"`
	HTTPStatus   int            `json:"http_status"`
	Metadata     map[string]any `json:"metadata"`
}

type Adapter interface {
	Name() string
	Fetch(ctx context.Context, since time.Time) ([]FetchedItem, error)
}

func RawPayload(v any) []byte { b, _ := json.Marshal(v); return b }

func SourceConfigValue[T any](src db.Source, key string, def T) T {
	if src.Config == nil {
		return def
	}
	v, ok := src.Config[key]
	if !ok {
		return def
	}
	b, _ := json.Marshal(v)
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return def
	}
	return out
}
