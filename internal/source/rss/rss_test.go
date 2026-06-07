package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

func TestRSSParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title><item><title>Fed holds rates</title><link>https://example.com/a?utm_source=x</link><description>Policy text with enough detail</description><pubDate>` + time.Now().UTC().Format(time.RFC1123Z) + `</pubDate><guid>1</guid></item></channel></rss>`))
	}))
	defer srv.Close()
	ad := New(db.Source{ID: uuid.New(), Key: "test", URL: srv.URL, FullContentAllowed: true}, srv.Client(), "ua")
	items, err := ad.Fetch(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Fed holds rates" {
		t.Fatalf("bad items %#v", items)
	}
}
