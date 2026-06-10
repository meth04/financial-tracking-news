package html

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/normalize"
	"github.com/nguyen/financial-tracking-news/internal/source"
)

type Adapter struct {
	Src       db.Source
	Client    *http.Client
	UserAgent string
}

type HTMLConfig struct {
	List                HTMLListConfig    `json:"list"`
	Article             HTMLArticleConfig `json:"article"`
	Filters             HTMLFilterConfig  `json:"filters"`
	DateFormats         []string          `json:"date_formats"`
	MaxItems            int               `json:"max_items"`
	MaxArticleFetches   int               `json:"max_article_fetches"`
	AllowMissingDates   bool              `json:"allow_missing_dates"`
	FallbackKeywordScan bool              `json:"fallback_keyword_scan"`
}

type HTMLListConfig struct {
	ItemSelector    string `json:"item_selector"`
	LinkSelector    string `json:"link_selector"`
	TitleSelector   string `json:"title_selector"`
	ExcerptSelector string `json:"excerpt_selector"`
	DateSelector    string `json:"date_selector"`
	DateAttr        string `json:"date_attr"`
}

type HTMLArticleConfig struct {
	TitleSelector   string   `json:"title_selector"`
	BodySelector    string   `json:"body_selector"`
	DateSelector    string   `json:"date_selector"`
	DateAttr        string   `json:"date_attr"`
	RemoveSelectors []string `json:"remove_selectors"`
}

type HTMLFilterConfig struct {
	IncludeURLPatterns   []string `json:"include_url_patterns"`
	ExcludeURLPatterns   []string `json:"exclude_url_patterns"`
	IncludeTitlePatterns []string `json:"include_title_patterns"`
	ExcludeTitlePatterns []string `json:"exclude_title_patterns"`
	AllowedDomains       []string `json:"allowed_domains"`
}

type candidate struct {
	URL          string
	CanonicalURL string
	Title        string
	Excerpt      string
	PublishedAt  *time.Time
	Metadata     map[string]any
}

func New(src db.Source, client *http.Client, ua string) *Adapter {
	return &Adapter{Src: src, Client: client, UserAgent: ua}
}
func (a *Adapter) Name() string { return a.Src.Key }

func (a *Adapter) Fetch(ctx context.Context, since time.Time) ([]source.FetchedItem, error) {
	res, err := a.FetchWithDiagnostics(ctx, since)
	return res.Items, err
}

func (a *Adapter) FetchWithDiagnostics(ctx context.Context, since time.Time) (source.FetchResult, error) {
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	cfg := loadConfig(a.Src)
	configured := cfg.hasSelectors()
	stats := map[string]any{"adapter": "selector_html", "list_url": a.Src.URL, "configured_selectors": configured, "since": since.UTC()}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.Src.URL, nil)
	if a.UserAgent != "" {
		req.Header.Set("User-Agent", a.UserAgent)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		stats["fetch_error"] = err.Error()
		return source.FetchResult{Metadata: stats}, err
	}
	defer resp.Body.Close()
	stats["http_status"] = resp.StatusCode
	stats["content_type"] = resp.Header.Get("Content-Type")
	if resp.StatusCode >= 400 {
		return source.FetchResult{Metadata: stats}, fmt.Errorf("html source status %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		stats["parse_error"] = err.Error()
		return source.FetchResult{Metadata: stats}, err
	}
	now := time.Now().UTC()
	seen := map[string]bool{}
	out := []source.FetchedItem{}
	base, _ := url.Parse(a.Src.URL)
	candidates := a.extractCandidates(doc, base, cfg, configured, stats)
	stats["candidate_links"] = len(candidates)
	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = 50
	}
	maxFetches := cfg.MaxArticleFetches
	if maxFetches <= 0 {
		maxFetches = maxItems
	}
	articleFetches := 0
	for _, cand := range candidates {
		if len(out) >= maxItems {
			inc(stats, "limited_by_max_items")
			break
		}
		if cand.CanonicalURL == "" {
			cand.CanonicalURL = normalize.CanonicalURL(cand.URL)
		}
		if cand.CanonicalURL == "" || seen[cand.CanonicalURL] {
			inc(stats, "duplicates_on_page")
			continue
		}
		seen[cand.CanonicalURL] = true
		if !allowedDomain(cand.URL, cfg.Filters.AllowedDomains) {
			inc(stats, "filtered_by_domain")
			continue
		}
		if !includeAllowed(cand.URL, cfg.Filters.IncludeURLPatterns) || matchesAny(cand.URL, cfg.Filters.ExcludeURLPatterns) {
			inc(stats, "filtered_by_url")
			continue
		}
		if !includeAllowed(cand.Title, cfg.Filters.IncludeTitlePatterns) || matchesAny(cand.Title, cfg.Filters.ExcludeTitlePatterns) {
			inc(stats, "filtered_by_title")
			continue
		}
		if !configured && !cfg.FallbackKeywordScan && !legacyRelevantTitle(cand.Title) {
			inc(stats, "filtered_by_legacy_keyword")
			continue
		}
		if cand.PublishedAt != nil && cand.PublishedAt.Before(since) {
			inc(stats, "older_than_window_count")
			continue
		}
		if cand.PublishedAt == nil {
			inc(stats, "missing_date_count")
			if configured && !cfg.AllowMissingDates {
				inc(stats, "missing_date_skipped_count")
				continue
			}
		}
		if a.Src.FullContentAllowed && articleFetches >= maxFetches {
			inc(stats, "limited_by_max_article_fetches")
			break
		}
		contentText := cand.Excerpt
		contentHTML := ""
		contentType := "text/html"
		httpStatus := resp.StatusCode
		metadata := map[string]any{"adapter": "selector_html", "list_url": a.Src.URL, "content_source": "listing", "configured_selectors": configured}
		for k, v := range cand.Metadata {
			metadata[k] = v
		}
		pub := cand.PublishedAt
		if a.Src.FullContentAllowed && articleFetches < maxFetches {
			articleFetches++
			inc(stats, "content_fetch_attempted")
			page, err := source.FetchReadableContentWithOptions(ctx, client, cand.URL, a.UserAgent, source.DefaultMaxArticleBytes, source.ContentExtractOptions{TitleSelector: cfg.Article.TitleSelector, DateSelector: cfg.Article.DateSelector, DateAttr: cfg.Article.DateAttr, BodySelector: cfg.Article.BodySelector, RemoveSelectors: cfg.Article.RemoveSelectors, DateFormats: cfg.DateFormats})
			if err != nil {
				metadata["full_content_error"] = err.Error()
				inc(stats, "content_fetch_failed")
			} else {
				if page.CanonicalURL != "" {
					cand.CanonicalURL = normalize.CanonicalURL(page.CanonicalURL)
				}
				if page.Title != "" && len([]rune(page.Title)) > len([]rune(cand.Title)) {
					cand.Title = page.Title
				}
				if page.PublishedAt != nil {
					pub = page.PublishedAt
					metadata["published_at_source"] = "article"
				}
				if page.ContentText != "" {
					contentText = page.ContentText
					contentHTML = page.ContentHTML
					metadata["content_source"] = "linked_page"
					metadata["full_content_fetched"] = true
				}
				if page.ContentType != "" {
					contentType = page.ContentType
				}
				if page.HTTPStatus != 0 {
					httpStatus = page.HTTPStatus
				}
				for k, v := range page.Metadata {
					metadata["page_"+k] = v
				}
			}
		}
		if pub != nil && pub.Before(since) {
			inc(stats, "older_than_window_count")
			continue
		}
		metadata["content_char_count"] = len([]rune(contentText))
		metadata["content_word_count"] = normalize.WordCount(contentText)
		if pub == nil {
			metadata["published_at_source"] = "missing"
		} else if _, ok := metadata["published_at_source"]; !ok {
			metadata["published_at_source"] = "list"
		}
		payload := map[string]any{"url": cand.URL, "canonical_url": cand.CanonicalURL, "title": cand.Title, "excerpt": cand.Excerpt, "content_text": contentText, "published_at": pub, "metadata": metadata}
		out = append(out, source.FetchedItem{SourceKey: a.Src.Key, SourceID: a.Src.ID, RawURL: cand.URL, CanonicalURL: cand.CanonicalURL, Title: cand.Title, Excerpt: cand.Excerpt, ContentHTML: contentHTML, ContentText: contentText, FetchedAt: now, PublishedAt: pub, RawPayload: source.RawPayload(payload), ContentType: contentType, HTTPStatus: httpStatus, Metadata: metadata})
	}
	stats["content_fetch_attempted"] = articleFetches
	stats["returned_fresh_count"] = len(out)
	if len(out) == 0 {
		stats["no_fresh_reason"] = noFreshReason(stats)
	}
	return source.FetchResult{Items: out, Metadata: stats}, nil
}

func loadConfig(src db.Source) HTMLConfig {
	cfg := source.SourceConfigValue(src, "html", HTMLConfig{})
	if cfg.MaxItems == 0 {
		cfg.MaxItems = source.SourceConfigValue(src, "max_items", 0)
	}
	if cfg.MaxArticleFetches == 0 {
		cfg.MaxArticleFetches = source.SourceConfigValue(src, "max_article_fetches", 0)
	}
	return cfg
}

func (c HTMLConfig) hasSelectors() bool {
	return strings.TrimSpace(c.List.ItemSelector+c.List.LinkSelector+c.List.TitleSelector+c.List.DateSelector+c.Article.BodySelector) != ""
}

func (a *Adapter) extractCandidates(doc *goquery.Document, base *url.URL, cfg HTMLConfig, configured bool, stats map[string]any) []candidate {
	out := []candidate{}
	selectionCount := 0
	if cfg.List.ItemSelector != "" {
		doc.Find(cfg.List.ItemSelector).Each(func(_ int, sel *goquery.Selection) {
			selectionCount++
			if cand, ok := a.extractCandidate(sel, base, cfg); ok {
				out = append(out, cand)
			}
		})
	} else if cfg.List.LinkSelector != "" {
		doc.Find(cfg.List.LinkSelector).Each(func(_ int, sel *goquery.Selection) {
			selectionCount++
			if cand, ok := a.extractCandidate(sel, base, cfg); ok {
				out = append(out, cand)
			}
		})
	} else {
		doc.Find("a").Each(func(_ int, sel *goquery.Selection) {
			selectionCount++
			if cand, ok := a.extractCandidate(sel, base, cfg); ok {
				out = append(out, cand)
			}
		})
	}
	stats["selector_matches"] = selectionCount
	stats["raw_candidate_count"] = len(out)
	stats["configured_html"] = configured
	return out
}

func (a *Adapter) extractCandidate(sel *goquery.Selection, base *url.URL, cfg HTMLConfig) (candidate, bool) {
	linkSel := sel
	if cfg.List.LinkSelector != "" {
		if found := sel.Find(cfg.List.LinkSelector).First(); found.Length() > 0 {
			linkSel = found
		}
	} else if goquery.NodeName(sel) != "a" {
		if found := sel.Find("a[href]").First(); found.Length() > 0 {
			linkSel = found
		}
	}
	href, ok := linkSel.Attr("href")
	if !ok || strings.TrimSpace(href) == "" {
		return candidate{}, false
	}
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return candidate{}, false
	}
	abs := base.ResolveReference(u).String()
	title := selectionText(sel, cfg.List.TitleSelector)
	if title == "" {
		title = strings.TrimSpace(linkSel.Text())
	}
	title = normalize.CleanText(title)
	if len([]rune(title)) < 8 {
		return candidate{}, false
	}
	excerpt := selectionText(sel, cfg.List.ExcerptSelector)
	pub, dateSource := selectionDate(sel, cfg.List.DateSelector, cfg.List.DateAttr, cfg.DateFormats)
	meta := map[string]any{"selector_link": cfg.List.LinkSelector, "selector_title": cfg.List.TitleSelector}
	if dateSource != "" {
		meta["published_at_source"] = "list"
		meta["date_selector_used"] = dateSource
	}
	return candidate{URL: abs, CanonicalURL: normalize.CanonicalURL(abs), Title: title, Excerpt: excerpt, PublishedAt: pub, Metadata: meta}, true
}

func selectionText(sel *goquery.Selection, selector string) string {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return ""
	}
	found := sel.Find(selector).First()
	if found.Length() == 0 {
		return ""
	}
	if attr, ok := found.Attr("content"); ok {
		return normalize.CleanText(attr)
	}
	return normalize.CleanText(found.Text())
}

func selectionDate(sel *goquery.Selection, selector, attr string, formats []string) (*time.Time, string) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, ""
	}
	var parsed *time.Time
	var used string
	sel.Find(selector).EachWithBreak(func(_ int, dsel *goquery.Selection) bool {
		candidates := []string{}
		if attr != "" {
			if v, ok := dsel.Attr(attr); ok {
				candidates = append(candidates, v)
			}
		}
		for _, a := range []string{"datetime", "content", "data-date"} {
			if v, ok := dsel.Attr(a); ok {
				candidates = append(candidates, v)
			}
		}
		candidates = append(candidates, dsel.Text())
		for _, c := range candidates {
			if t, ok := source.ParseDate(c, formats); ok {
				parsed = &t
				used = selector
				return false
			}
		}
		return true
	})
	return parsed, used
}

func allowedDomain(raw string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range allowed {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" && (host == domain || strings.HasSuffix(host, "."+domain)) {
			return true
		}
	}
	return false
}

func includeAllowed(s string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	return matchesAny(s, patterns)
}

func matchesAny(s string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if ok, err := regexp.MatchString(p, s); err == nil && ok {
			return true
		}
		if strings.Contains(strings.ToLower(s), strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func legacyRelevantTitle(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "release") || strings.Contains(low, "statement") || strings.Contains(low, "filing") || strings.Contains(low, "inflation") || strings.Contains(low, "rates") || strings.Contains(low, "treasury") || strings.Contains(low, "sec")
}

func inc(stats map[string]any, key string) {
	if stats == nil {
		return
	}
	if v, ok := stats[key]; ok {
		switch n := v.(type) {
		case int:
			stats[key] = n + 1
		case int64:
			stats[key] = n + 1
		case float64:
			stats[key] = n + 1
		default:
			stats[key] = 1
		}
		return
	}
	stats[key] = 1
}

func count(stats map[string]any, key string) int {
	if stats == nil {
		return 0
	}
	switch v := stats[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func noFreshReason(stats map[string]any) string {
	if count(stats, "selector_matches") == 0 || count(stats, "raw_candidate_count") == 0 {
		return "no_selector_matches"
	}
	if count(stats, "older_than_window_count") > 0 && count(stats, "candidate_links") == count(stats, "older_than_window_count") {
		return "all_matching_items_older_than_72h"
	}
	if count(stats, "filtered_by_url") > 0 || count(stats, "filtered_by_title") > 0 || count(stats, "filtered_by_domain") > 0 {
		return "all_candidates_filtered"
	}
	if count(stats, "missing_date_skipped_count") > 0 {
		return "matching_items_missing_dates"
	}
	if count(stats, "content_fetch_failed") > 0 {
		return "content_fetch_failed"
	}
	return "source_reachable_no_recent_items"
}
