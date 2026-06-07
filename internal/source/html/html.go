package html

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

func New(src db.Source, client *http.Client, ua string) *Adapter {
	return &Adapter{Src: src, Client: client, UserAgent: ua}
}
func (a *Adapter) Name() string { return a.Src.Key }

func (a *Adapter) Fetch(ctx context.Context, since time.Time) ([]source.FetchedItem, error) {
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.Src.URL, nil)
	if a.UserAgent != "" {
		req.Header.Set("User-Agent", a.UserAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("html source status %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	seen := map[string]bool{}
	out := []source.FetchedItem{}
	base, _ := url.Parse(a.Src.URL)
	doc.Find("a").Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			return
		}
		text := strings.TrimSpace(sel.Text())
		if len(text) < 12 {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		abs := base.ResolveReference(u).String()
		can := normalize.CanonicalURL(abs)
		if seen[can] {
			return
		}
		seen[can] = true
		low := strings.ToLower(text)
		if !(strings.Contains(low, "release") || strings.Contains(low, "statement") || strings.Contains(low, "filing") || strings.Contains(low, "inflation") || strings.Contains(low, "rates") || strings.Contains(low, "treasury") || strings.Contains(low, "sec")) {
			return
		}
		content := ""
		if a.Src.FullContentAllowed {
			content = text
		}
		out = append(out, source.FetchedItem{SourceKey: a.Src.Key, SourceID: a.Src.ID, RawURL: abs, CanonicalURL: can, Title: text, Excerpt: text, ContentText: content, FetchedAt: now, RawPayload: source.RawPayload(map[string]any{"url": abs, "title": text}), ContentType: "text/html", HTTPStatus: resp.StatusCode, Metadata: map[string]any{"adapter": "generic_html"}})
	})
	return out, nil
}
