package rss

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Src.URL, nil)
	if err != nil {
		return nil, err
	}
	if a.UserAgent != "" {
		req.Header.Set("User-Agent", a.UserAgent)
	}
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rss source status %d", resp.StatusCode)
	}

	parser := gofeed.NewParser()
	feed, err := parser.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]source.FetchedItem, 0, len(feed.Items))
	for _, it := range feed.Items {
		pub := it.PublishedParsed
		if pub == nil {
			pub = it.UpdatedParsed
		}
		if pub != nil && pub.Before(since) {
			continue
		}
		link := it.Link
		if link == "" {
			link = it.GUID
		}
		contentHTML := it.Content
		contentSource := "feed_content"
		if contentHTML == "" {
			contentHTML = it.Description
			contentSource = "feed_description"
		}
		contentText := normalize.CleanText(contentHTML)
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/rss+xml"
		}
		httpStatus := resp.StatusCode
		metadata := map[string]any{"guid": it.GUID, "feed_title": feed.Title, "content_source": contentSource}
		if a.Src.FullContentAllowed && link != "" {
			page, err := source.FetchReadableContent(ctx, client, link, a.UserAgent, source.DefaultMaxArticleBytes)
			if err != nil {
				metadata["full_content_error"] = err.Error()
			} else if page.ContentText != "" {
				contentText = page.ContentText
				contentHTML = page.ContentHTML
				contentSource = "linked_page"
				if page.ContentType != "" {
					contentType = page.ContentType
				}
				if page.HTTPStatus != 0 {
					httpStatus = page.HTTPStatus
				}
				metadata["full_content_fetched"] = true
				metadata["content_source"] = contentSource
			}
		} else if !a.Src.FullContentAllowed {
			contentHTML = ""
			contentText = ""
			metadata["content_source"] = "summary_disabled"
		}
		metadata["content_char_count"] = len([]rune(contentText))
		metadata["content_word_count"] = normalize.WordCount(contentText)
		payload := map[string]any{"title": it.Title, "link": link, "published": it.Published, "updated": it.Updated, "description": it.Description, "guid": it.GUID, "content_text": contentText}
		item := source.FetchedItem{SourceKey: a.Src.Key, SourceID: a.Src.ID, RawURL: link, CanonicalURL: normalize.CanonicalURL(link), Title: it.Title, Excerpt: it.Description, ContentHTML: contentHTML, ContentText: contentText, FetchedAt: now, PublishedAt: pub, RawPayload: source.RawPayload(payload), ContentType: contentType, HTTPStatus: httpStatus, Metadata: metadata}
		if it.Author != nil {
			item.Author = it.Author.Name
		}
		out = append(out, item)
	}
	return out, nil
}
