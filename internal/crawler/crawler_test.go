package crawler

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/source"
)

func TestAssessItemQualityRejectsShortFullContent(t *testing.T) {
	src := db.Source{ID: uuid.New(), Key: "official", FullContentAllowed: true, Config: map[string]any{"min_content_chars": 300, "min_word_count": 45, "require_article_content": true}}
	item := source.FetchedItem{Title: "Official market policy update", RawURL: "https://example.gov/news/1", ContentText: "Too short."}
	decision := assessItemQuality(src, item)
	if decision.OK {
		t.Fatalf("expected short full-content article to be rejected: %#v", decision)
	}
	if decision.Reason != "content below article-quality threshold" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestAssessItemQualityAcceptsArticleLengthContent(t *testing.T) {
	src := db.Source{ID: uuid.New(), Key: "official", FullContentAllowed: true, Config: map[string]any{"min_content_chars": 300, "min_word_count": 45, "require_article_content": true}}
	item := source.FetchedItem{Title: "Official market policy update", RawURL: "https://example.gov/news/1", ContentText: strings.Repeat("Market participants received a detailed official policy update. ", 12)}
	decision := assessItemQuality(src, item)
	if !decision.OK {
		t.Fatalf("expected article-length content to pass: %#v", decision)
	}
	if decision.ContentChars < 300 || decision.WordCount < 45 {
		t.Fatalf("expected useful content counts, got chars=%d words=%d", decision.ContentChars, decision.WordCount)
	}
}

func TestAssessItemQualityRejectsNavigationTitle(t *testing.T) {
	src := db.Source{ID: uuid.New(), Key: "official", FullContentAllowed: false, Config: map[string]any{}}
	item := source.FetchedItem{Title: "Subscribe to RSS", RawURL: "https://example.gov/rss", Excerpt: "Latest updates."}
	decision := assessItemQuality(src, item)
	if decision.OK {
		t.Fatalf("expected navigation title to be rejected: %#v", decision)
	}
}

func TestAssessItemQualityAllowsSummaryOnlySource(t *testing.T) {
	src := db.Source{ID: uuid.New(), Key: "summary", FullContentAllowed: false, Config: map[string]any{}}
	item := source.FetchedItem{Title: "Bank regulator issues market update", RawURL: "https://example.gov/news/2", Excerpt: "Short official summary."}
	decision := assessItemQuality(src, item)
	if !decision.OK {
		t.Fatalf("expected summary-only source to pass short content: %#v", decision)
	}
}
