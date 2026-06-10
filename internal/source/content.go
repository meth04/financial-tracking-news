package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/nguyen/financial-tracking-news/internal/normalize"
)

const DefaultMaxArticleBytes int64 = 2 << 20

// PageContent is the readable body extracted from a public article page.
type PageContent struct {
	ContentHTML  string         `json:"content_html"`
	ContentText  string         `json:"content_text"`
	ContentType  string         `json:"content_type"`
	HTTPStatus   int            `json:"http_status"`
	Title        string         `json:"title,omitempty"`
	CanonicalURL string         `json:"canonical_url,omitempty"`
	PublishedAt  *time.Time     `json:"published_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type ContentExtractOptions struct {
	TitleSelector   string   `json:"title_selector"`
	DateSelector    string   `json:"date_selector"`
	DateAttr        string   `json:"date_attr"`
	BodySelector    string   `json:"body_selector"`
	RemoveSelectors []string `json:"remove_selectors"`
	DateFormats     []string `json:"date_formats"`
}

// FetchReadableContent downloads a public HTML page and extracts the most likely
// article body. It is only called by adapters for sources whose configuration
// explicitly allows full-content crawling.
func FetchReadableContent(ctx context.Context, client *http.Client, rawURL, userAgent string, maxBytes int64) (PageContent, error) {
	return FetchReadableContentWithOptions(ctx, client, rawURL, userAgent, maxBytes, ContentExtractOptions{})
}

func FetchReadableContentWithOptions(ctx context.Context, client *http.Client, rawURL, userAgent string, maxBytes int64, opts ContentExtractOptions) (PageContent, error) {
	if strings.TrimSpace(rawURL) == "" {
		return PageContent{}, nil
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxArticleBytes
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return PageContent{}, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return PageContent{}, err
	}
	defer resp.Body.Close()

	out := PageContent{HTTPStatus: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("article page status %d", resp.StatusCode)
	}
	contentType := strings.ToLower(out.ContentType)
	if contentType != "" && !strings.Contains(contentType, "html") && !strings.Contains(contentType, "xml") && !strings.Contains(contentType, "text/plain") {
		return out, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return out, err
	}
	page, err := ExtractReadableContentWithOptions(body, opts)
	if err != nil {
		return out, err
	}
	page.HTTPStatus = out.HTTPStatus
	page.ContentType = out.ContentType
	return page, nil
}

// ExtractReadableContent returns cleaned article text and the selected HTML body.
func ExtractReadableContent(body []byte) (string, string, error) {
	page, err := ExtractReadableContentWithOptions(body, ContentExtractOptions{})
	return page.ContentText, page.ContentHTML, err
}

func ExtractReadableContentWithOptions(body []byte, opts ContentExtractOptions) (PageContent, error) {
	out := PageContent{Metadata: map[string]any{}}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		text := normalize.CleanText(string(body))
		out.ContentText = truncateUTF8(text, 40_000)
		return out, nil
	}
	removeJunk(doc, opts.RemoveSelectors)
	out.Title = firstText(doc, opts.TitleSelector)
	if out.Title == "" {
		out.Title = firstText(doc, "meta[property='og:title'], meta[name='twitter:title'], h1, title")
	}
	out.CanonicalURL = firstAttr(doc, "link[rel='canonical']", "href")
	if out.CanonicalURL == "" {
		out.CanonicalURL = firstAttr(doc, "meta[property='og:url']", "content")
	}
	if opts.DateSelector != "" {
		if t, src, ok := extractTime(doc, opts.DateSelector, opts.DateAttr, opts.DateFormats); ok {
			out.PublishedAt = &t
			out.Metadata["date_source"] = src
		}
	}
	best, selectorUsed := bestBodySelection(doc, opts.BodySelector)
	if best == nil {
		text := normalize.CleanText(doc.Text())
		out.ContentText = truncateUTF8(text, 40_000)
		return out, nil
	}
	text := normalize.CleanText(best.Text())
	html, _ := best.Html()
	out.ContentText = truncateUTF8(text, 40_000)
	out.ContentHTML = truncateUTF8(html, 120_000)
	out.Metadata["body_selector_used"] = selectorUsed
	out.Metadata["content_word_count"] = normalize.WordCount(out.ContentText)
	out.Metadata["content_char_count"] = len([]rune(out.ContentText))
	return out, nil
}

func removeJunk(doc *goquery.Document, extra []string) {
	selectors := []string{"script", "style", "noscript", "svg", "iframe", "form", "nav", "header", "footer", "aside"}
	selectors = append(selectors, extra...)
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector != "" {
			doc.Find(selector).Remove()
		}
	}
}

func bestBodySelection(doc *goquery.Document, configured string) (*goquery.Selection, string) {
	selectors := splitSelectors(configured)
	configuredMode := len(selectors) > 0
	if len(selectors) == 0 {
		selectors = []string{"article", "main article", "main", "[role='main']", ".article-body", ".article__body", ".story-body", ".entry-content", ".post-content", ".press-release", ".field-name-body", "#content", ".content", "body"}
	}
	var best *goquery.Selection
	bestWords := 0
	bestSelector := ""
	for _, selector := range selectors {
		var selectorBest *goquery.Selection
		selectorBestWords := 0
		doc.Find(selector).Each(func(_ int, sel *goquery.Selection) {
			text := normalize.CleanText(sel.Text())
			words := normalize.WordCount(text)
			if words > selectorBestWords {
				selectorBest = sel.Clone()
				selectorBestWords = words
			}
		})
		if configuredMode && selectorBestWords >= 20 {
			return selectorBest, selector
		}
		if selectorBestWords > bestWords {
			best = selectorBest
			bestWords = selectorBestWords
			bestSelector = selector
		}
		if !configuredMode && bestWords >= 120 {
			break
		}
	}
	return best, bestSelector
}

func splitSelectors(selectors string) []string {
	parts := strings.Split(selectors, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstText(doc *goquery.Document, selector string) string {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return ""
	}
	var text string
	doc.Find(selector).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		if attr, ok := sel.Attr("content"); ok {
			text = strings.TrimSpace(attr)
		} else {
			text = strings.TrimSpace(sel.Text())
		}
		return text == ""
	})
	return normalize.CleanText(text)
}

func firstAttr(doc *goquery.Document, selector, attr string) string {
	selector = strings.TrimSpace(selector)
	if selector == "" || attr == "" {
		return ""
	}
	var out string
	doc.Find(selector).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		out, _ = sel.Attr(attr)
		out = strings.TrimSpace(out)
		return out == ""
	})
	return out
}

func extractTime(doc *goquery.Document, selector, attr string, formats []string) (time.Time, string, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return time.Time{}, "", false
	}
	var parsed time.Time
	var source string
	found := false
	doc.Find(selector).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		candidates := []string{}
		if attr != "" {
			if v, ok := sel.Attr(attr); ok {
				candidates = append(candidates, v)
			}
		}
		for _, a := range []string{"datetime", "content", "data-date"} {
			if v, ok := sel.Attr(a); ok {
				candidates = append(candidates, v)
			}
		}
		candidates = append(candidates, sel.Text())
		for _, candidate := range candidates {
			if t, ok := ParseDate(candidate, formats); ok {
				parsed = t
				source = selector
				found = true
				return false
			}
		}
		return true
	})
	return parsed, source, found
}

func ParseDate(raw string, formats []string) (time.Time, bool) {
	s := normalize.CleanText(strings.TrimSpace(raw))
	if s == "" {
		return time.Time{}, false
	}
	s = strings.TrimPrefix(s, "Release Date:")
	s = strings.TrimPrefix(s, "Released:")
	s = strings.TrimPrefix(s, "Published:")
	s = strings.TrimSpace(strings.Trim(s, "|·-"))
	layouts := append([]string{}, formats...)
	layouts = append(layouts,
		time.RFC3339, time.RFC1123, time.RFC1123Z, "2006-01-02", "2006/01/02", "01/02/2006",
		"January 2, 2006", "Jan 2, 2006", "Jan. 2, 2006", "Monday, January 2, 2006",
		"Jan 02, 2006", "Jan. 02, 2006", "02 Jan 2006",
	)
	for _, layout := range layouts {
		if layout == "" {
			continue
		}
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	if len(s) >= 10 {
		for _, part := range []string{s[:10], strings.TrimSpace(strings.Split(s, "|")[0])} {
			if part == "" || part == s {
				continue
			}
			if t, ok := ParseDate(part, formats); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func truncateUTF8(s string, max int) string {
	if len(s) <= max || max <= 0 {
		return s
	}
	if max >= len(s) {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut <= 0 {
		return ""
	}
	return strings.TrimSpace(s[:cut])
}
