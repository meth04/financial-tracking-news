package source

import (
	"strings"
	"testing"
)

func TestExtractReadableContentWithOptionsUsesConfiguredBody(t *testing.T) {
	body := []byte(`<html><head><title>Fallback title</title><link rel="canonical" href="https://example.gov/news/1"></head><body><nav>` + strings.Repeat("navigation ", 100) + `</nav><main><h1>Official release title</h1><time datetime="2026-06-10">June 10, 2026</time><section class="release-body"><p>` + strings.Repeat("This official economic release contains market relevant details. ", 12) + `</p></section></main><footer>footer</footer></body></html>`)
	page, err := ExtractReadableContentWithOptions(body, ContentExtractOptions{TitleSelector: "h1", DateSelector: "time", DateAttr: "datetime", BodySelector: ".release-body", RemoveSelectors: []string{"nav", "footer"}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Official release title" {
		t.Fatalf("title = %q", page.Title)
	}
	if page.CanonicalURL != "https://example.gov/news/1" {
		t.Fatalf("canonical = %q", page.CanonicalURL)
	}
	if page.PublishedAt == nil {
		t.Fatal("expected published date")
	}
	if !strings.Contains(page.ContentText, "market relevant details") || strings.Contains(page.ContentText, "navigation") {
		t.Fatalf("unexpected extracted text %q", page.ContentText)
	}
	if page.Metadata["body_selector_used"] != ".release-body" {
		t.Fatalf("expected configured selector metadata, got %#v", page.Metadata)
	}
}

func TestParseDateCommonFormats(t *testing.T) {
	for _, raw := range []string{"2026-06-10", "June 10, 2026", "Jan. 2, 2026", "Release Date: 01/02/2026"} {
		if _, ok := ParseDate(raw, nil); !ok {
			t.Fatalf("failed to parse %q", raw)
		}
	}
}
