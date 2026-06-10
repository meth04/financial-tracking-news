package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

func TestRSSParsingSendsUserAgentAndExtractsFullContent(t *testing.T) {
	var feedUA, articleUA string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			feedUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title><item><title>Fed holds rates</title><link>` + srv.URL + `/article</link><description>Policy text with enough detail</description><pubDate>` + time.Now().UTC().Format(time.RFC1123Z) + `</pubDate><guid>1</guid></item></channel></rss>`))
		case "/article":
			articleUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><nav>subscribe</nav><main><article><h1>Fed holds rates</h1><p>` + strings.Repeat("Monetary policy statement with market relevant details. ", 20) + `</p></article></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ad := New(db.Source{ID: uuid.New(), Key: "test", URL: srv.URL + "/feed.xml", FullContentAllowed: true}, srv.Client(), "test-agent")
	items, err := ad.Fetch(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Fed holds rates" {
		t.Fatalf("bad items %#v", items)
	}
	if feedUA != "test-agent" {
		t.Fatalf("feed user-agent = %q", feedUA)
	}
	if articleUA != "test-agent" {
		t.Fatalf("article user-agent = %q", articleUA)
	}
	if !strings.Contains(items[0].ContentText, "Monetary policy statement") {
		t.Fatalf("expected linked article content, got %q", items[0].ContentText)
	}
	if items[0].Metadata["full_content_fetched"] != true {
		t.Fatalf("expected full_content_fetched metadata, got %#v", items[0].Metadata)
	}
	if got, ok := items[0].Metadata["content_word_count"].(int); !ok || got < 50 {
		t.Fatalf("expected content_word_count metadata >= 50, got %#v", items[0].Metadata["content_word_count"])
	}
}
