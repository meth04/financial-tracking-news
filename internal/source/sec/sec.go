package sec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

type tickerInfo struct {
	CIK    int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}
type submissions struct {
	Name    string   `json:"name"`
	Tickers []string `json:"tickers"`
	Filings struct {
		Recent recentFilings `json:"recent"`
	} `json:"filings"`
}
type recentFilings struct {
	Form            []string `json:"form"`
	FilingDate      []string `json:"filingDate"`
	AccessionNumber []string `json:"accessionNumber"`
	PrimaryDocument []string `json:"primaryDocument"`
}

func KeepForm(form string, allowed []string) bool {
	for _, f := range allowed {
		if strings.EqualFold(strings.TrimSpace(f), form) {
			return true
		}
	}
	return false
}

func (a *Adapter) Fetch(ctx context.Context, since time.Time) ([]source.FetchedItem, error) {
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	tickers := source.SourceConfigValue(a.Src, "tickers", []string{})
	forms := source.SourceConfigValue(a.Src, "forms", []string{"8-K", "10-Q", "10-K"})
	tickerURL := source.SourceConfigValue(a.Src, "company_tickers_url", "https://www.sec.gov/files/company_tickers.json")
	mapping, err := a.loadTickers(ctx, client, tickerURL)
	if err != nil {
		return nil, err
	}
	out := []source.FetchedItem{}
	now := time.Now().UTC()
	for _, sym := range tickers {
		info, ok := mapping[strings.ToUpper(sym)]
		if !ok {
			continue
		}
		url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%010d.json", info.CIK)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if a.UserAgent != "" {
			req.Header.Set("User-Agent", a.UserAgent)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		var sub submissions
		decErr := json.NewDecoder(resp.Body).Decode(&sub)
		resp.Body.Close()
		if resp.StatusCode >= 400 || decErr != nil {
			continue
		}
		for i, form := range sub.Filings.Recent.Form {
			if !KeepForm(form, forms) || i >= len(sub.Filings.Recent.FilingDate) {
				continue
			}
			pub, err := time.Parse("2006-01-02", sub.Filings.Recent.FilingDate[i])
			if err != nil || pub.Before(since) {
				continue
			}
			acc := ""
			if i < len(sub.Filings.Recent.AccessionNumber) {
				acc = sub.Filings.Recent.AccessionNumber[i]
			}
			doc := ""
			if i < len(sub.Filings.Recent.PrimaryDocument) {
				doc = sub.Filings.Recent.PrimaryDocument[i]
			}
			link := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s", info.CIK, strings.ReplaceAll(acc, "-", ""), doc)
			title := fmt.Sprintf("%s %s filed %s", strings.ToUpper(sym), form, pub.Format("2006-01-02"))
			excerpt := fmt.Sprintf("%s filed form %s with the SEC.", info.Title, form)
			contentText := fmt.Sprintf("%s filed form %s with the SEC on %s. Accession %s.", info.Title, form, pub.Format("2006-01-02"), acc)
			contentHTML := ""
			contentType := "application/json"
			httpStatus := 200
			metadata := map[string]any{"ticker": strings.ToUpper(sym), "form": form, "accession": acc}
			if a.Src.FullContentAllowed && doc != "" {
				page, err := source.FetchReadableContent(ctx, client, link, a.UserAgent, source.DefaultMaxArticleBytes)
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
			out = append(out, source.FetchedItem{SourceKey: a.Src.Key, SourceID: a.Src.ID, RawURL: link, CanonicalURL: normalize.CanonicalURL(link), Title: title, Excerpt: excerpt, ContentHTML: contentHTML, ContentText: contentText, PublishedAt: &pub, FetchedAt: now, RawPayload: source.RawPayload(map[string]any{"ticker": sym, "form": form, "date": pub, "accession": acc, "document": doc, "content_text": contentText}), ContentType: contentType, HTTPStatus: httpStatus, Metadata: metadata})
		}
	}
	return out, nil
}

func (a *Adapter) loadTickers(ctx context.Context, client *http.Client, url string) (map[string]tickerInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if a.UserAgent != "" {
		req.Header.Set("User-Agent", a.UserAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sec tickers status %d", resp.StatusCode)
	}
	var raw map[string]tickerInfo
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := map[string]tickerInfo{}
	for _, info := range raw {
		out[strings.ToUpper(info.Ticker)] = info
	}
	return out, nil
}
