package html

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

func TestHTMLAdapterUsesConfiguredSelectorsAndArticleBody(t *testing.T) {
	var listUA, articleUA string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/news":
			listUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
				<nav><a href="/subscribe">Subscribe</a></nav>
				<section class="list">
				  <article class="item"><a class="headline" href="/news/market-update">Agencies request comment on market data rule</a><time datetime="` + time.Now().UTC().Format("2006-01-02") + `">Today</time><p class="summary">Summary text</p></article>
				  <article class="item"><a class="headline" href="/news/old-rule">Old market data rule</a><time datetime="2020-01-02">Old</time></article>
				</section></body></html>`))
		case "/news/market-update":
			articleUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><link rel="canonical" href="` + srv.URL + `/news/market-update"></head><body><header>Nav</header><main><article><h1>Agencies request comment on market data rule</h1><div class="body"><p>` + strings.Repeat("This official release explains a market data rule and its expected effects. ", 14) + `</p></div></article></main><footer>Footer</footer></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	src := db.Source{ID: uuid.New(), Key: "html_test", URL: srv.URL + "/news", FullContentAllowed: true, Config: map[string]any{
		"html": map[string]any{
			"list":                map[string]any{"item_selector": ".item", "link_selector": "a.headline", "title_selector": "a.headline", "date_selector": "time", "date_attr": "datetime", "excerpt_selector": ".summary"},
			"article":             map[string]any{"title_selector": "h1", "body_selector": ".body", "remove_selectors": []string{"header", "footer"}},
			"filters":             map[string]any{"include_url_patterns": []string{"/news/"}, "exclude_url_patterns": []string{"subscribe"}, "allowed_domains": []string{"127.0.0.1"}},
			"max_items":           10,
			"max_article_fetches": 5,
			"allow_missing_dates": false,
		},
	}}
	ad := New(src, srv.Client(), "html-test-agent")
	res, err := ad.FetchWithDiagnostics(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected one fresh item, got %d metadata=%#v", len(res.Items), res.Metadata)
	}
	if listUA != "html-test-agent" || articleUA != "html-test-agent" {
		t.Fatalf("missing user-agent list=%q article=%q", listUA, articleUA)
	}
	item := res.Items[0]
	if item.Title != "Agencies request comment on market data rule" {
		t.Fatalf("unexpected title %q", item.Title)
	}
	if item.PublishedAt == nil {
		t.Fatal("expected parsed published date")
	}
	if !strings.Contains(item.ContentText, "market data rule") || len(item.ContentText) < 300 {
		t.Fatalf("expected article body content, got %q", item.ContentText)
	}
	if res.Metadata["older_than_window_count"] == nil {
		t.Fatalf("expected old item diagnostic metadata, got %#v", res.Metadata)
	}
}

func TestHTMLAdapterFallbackKeepsLegacyKeywordFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<a href="/release">Important rate release</a><a href="/about">About this site</a>`))
			return
		}
		_, _ = w.Write([]byte(`<main><p>` + strings.Repeat("Official release content. ", 30) + `</p></main>`))
	}))
	defer srv.Close()
	src := db.Source{ID: uuid.New(), Key: "legacy", URL: srv.URL, FullContentAllowed: true}
	items, err := New(src, srv.Client(), "ua").Fetch(context.Background(), time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Important rate release" {
		t.Fatalf("expected only legacy keyword matching release, got %#v", items)
	}
}
