package rss

import (
	"context"
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
	parser := gofeed.NewParser()
	if a.Client != nil {
		parser.Client = a.Client
	}
	feed, err := parser.ParseURLWithContext(a.Src.URL, ctx)
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
		content := it.Content
		if content == "" {
			content = it.Description
		}
		if !a.Src.FullContentAllowed {
			content = ""
		}
		link := it.Link
		if link == "" {
			link = it.GUID
		}
		payload := map[string]any{"title": it.Title, "link": link, "published": it.Published, "updated": it.Updated, "description": it.Description, "guid": it.GUID}
		item := source.FetchedItem{SourceKey: a.Src.Key, SourceID: a.Src.ID, RawURL: link, CanonicalURL: normalize.CanonicalURL(link), Title: it.Title, Excerpt: it.Description, ContentHTML: content, ContentText: normalize.CleanText(content), FetchedAt: now, PublishedAt: pub, RawPayload: source.RawPayload(payload), ContentType: "application/rss+xml", HTTPStatus: 200, Metadata: map[string]any{"guid": it.GUID, "feed_title": feed.Title}}
		if it.Author != nil {
			item.Author = it.Author.Name
		}
		out = append(out, item)
	}
	return out, nil
}
