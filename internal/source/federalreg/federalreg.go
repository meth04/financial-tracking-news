package federalreg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/normalize"
	"github.com/nguyen/financial-tracking-news/internal/source"
)

type Adapter struct {
	Src    db.Source
	Client *http.Client
}

func New(src db.Source, client *http.Client) *Adapter { return &Adapter{Src: src, Client: client} }
func (a *Adapter) Name() string                       { return a.Src.Key }

type response struct {
	Results []doc `json:"results"`
}
type doc struct {
	Title           string `json:"title"`
	Abstract        string `json:"abstract"`
	HTMLURL         string `json:"html_url"`
	PDFURL          string `json:"pdf_url"`
	Type            string `json:"type"`
	PublicationDate string `json:"publication_date"`
	Agencies        []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"agencies"`
}

func BuildURL(base string, since time.Time, agencies []string, types []string) string {
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("per_page", "100")
	q.Set("order", "newest")
	q.Set("conditions[publication_date][gte]", since.Format("2006-01-02"))
	for _, ag := range agencies {
		q.Add("conditions[agencies][]", ag)
	}
	for _, typ := range types {
		q.Add("conditions[type][]", typ)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (a *Adapter) Fetch(ctx context.Context, since time.Time) ([]source.FetchedItem, error) {
	agencies := source.SourceConfigValue(a.Src, "agencies", []string{})
	conds := source.SourceConfigValue(a.Src, "conditions", map[string][]string{})
	types := conds["type"]
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, BuildURL(a.Src.URL, since, agencies, types), nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("federal register status %d", resp.StatusCode)
	}
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := []source.FetchedItem{}
	for _, d := range r.Results {
		pub, _ := time.Parse("2006-01-02", d.PublicationDate)
		if !pub.IsZero() && pub.Before(since) {
			continue
		}
		link := d.HTMLURL
		if link == "" {
			link = d.PDFURL
		}
		names := []string{}
		for _, ag := range d.Agencies {
			names = append(names, ag.Name)
		}
		contentText := d.Abstract
		contentHTML := ""
		contentType := "application/json"
		httpStatus := 200
		metadata := map[string]any{"type": d.Type, "agencies": strings.Join(names, ",")}
		if a.Src.FullContentAllowed && d.HTMLURL != "" {
			page, err := source.FetchReadableContent(ctx, client, d.HTMLURL, "", source.DefaultMaxArticleBytes)
			if err != nil {
				metadata["full_content_error"] = err.Error()
			} else if page.ContentText != "" {
				contentText = page.ContentText
				contentHTML = page.ContentHTML
				if page.ContentType != "" {
					contentType = page.ContentType
				}
				if page.HTTPStatus != 0 {
					httpStatus = page.HTTPStatus
				}
				metadata["full_content_fetched"] = true
			}
		}
		out = append(out, source.FetchedItem{SourceKey: a.Src.Key, SourceID: a.Src.ID, RawURL: link, CanonicalURL: normalize.CanonicalURL(link), Title: d.Title, Excerpt: d.Abstract, ContentHTML: contentHTML, ContentText: contentText, PublishedAt: &pub, FetchedAt: now, RawPayload: source.RawPayload(map[string]any{"document": d, "content_text": contentText}), ContentType: contentType, HTTPStatus: httpStatus, Metadata: metadata})
	}
	return out, nil
}
