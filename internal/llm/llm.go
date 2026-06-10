package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nguyen/financial-tracking-news/internal/db"
)

type Analyzer interface {
	AnalyzeArticle(ctx context.Context, article db.Article) (*db.Analysis, string, error)
}

type OpenAIClient struct {
	BaseURL, APIKey, Model     string
	HTTP                       *http.Client
	SystemPrompt, UserTemplate string
}

func NewOpenAIClient(baseURL, apiKey, model string, timeout time.Duration) *OpenAIClient {
	return &OpenAIClient{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, HTTP: &http.Client{Timeout: timeout}, SystemPrompt: loadPrompt("prompts/article_analysis_system.md", defaultSystemPrompt), UserTemplate: loadPrompt("prompts/article_analysis_user_template.md", defaultUserTemplate)}
}

func (c *OpenAIClient) AnalyzeArticle(ctx context.Context, article db.Article) (*db.Analysis, string, error) {
	prompt := RenderPrompt(c.UserTemplate, article)
	reqBody := map[string]any{"model": c.Model, "temperature": 0.1, "messages": []map[string]string{{"role": "system", "content": c.SystemPrompt}, {"role": "user", "content": prompt}}}
	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, string(body), fmt.Errorf("llm status %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, string(body), err
	}
	if len(parsed.Choices) == 0 {
		return nil, string(body), errors.New("llm returned no choices")
	}
	raw := parsed.Choices[0].Message.Content
	ana, err := ParseAnalysis(raw)
	if err != nil {
		return nil, raw, err
	}
	ana.Model = c.Model
	ana.ArticleID = article.ID
	return ana, raw, nil
}

func RenderPrompt(t string, a db.Article) string {
	publishedAt := ""
	if a.PublishedAt != nil {
		publishedAt = a.PublishedAt.Format(time.RFC3339)
	}
	canonicalURL := ""
	if a.CanonicalURL != nil {
		canonicalURL = *a.CanonicalURL
	}
	author := ""
	if a.Author != nil {
		author = *a.Author
	}
	excerpt := ""
	if a.Excerpt != nil {
		excerpt = *a.Excerpt
	}
	contentText := ""
	if a.ContentText != nil {
		contentText = *a.ContentText
	}
	contentHTML := ""
	if a.ContentHTML != nil {
		contentHTML = *a.ContentHTML
	}

	repl := map[string]string{
		"{{title}}":                    a.Title,
		"{{normalized_title}}":         a.NormalizedTitle,
		"{{source}}":                   a.SourceName,
		"{{source_name}}":              a.SourceName,
		"{{source_key}}":               a.SourceKey,
		"{{source_credibility_score}}": strconv.Itoa(a.SourceCredibilityScore),
		"{{canonical_url}}":            canonicalURL,
		"{{author}}":                   author,
		"{{published_at}}":             publishedAt,
		"{{fetched_at}}":               a.FetchedAt.Format(time.RFC3339),
		"{{time_confidence}}":          a.TimeConfidence,
		"{{status}}":                   a.Status,
		"{{excerpt}}":                  excerpt,
		"{{content}}":                  contentText,
		"{{content_text}}":             contentText,
		"{{content_html}}":             contentHTML,
		"{{word_count}}":               strconv.Itoa(a.WordCount),
	}
	out := t
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	if !strings.Contains(t, "{{title}}") {
		out = fmt.Sprintf("Title: %s\nSource: %s\nPublished: %s\nFetched: %s\nURL: %s\nExcerpt: %s\nContent: %s", repl["{{title}}"], repl["{{source_name}}"], repl["{{published_at}}"], repl["{{fetched_at}}"], repl["{{canonical_url}}"], repl["{{excerpt}}"], repl["{{content_text}}"])
	}
	return out
}

type rawAnalysis struct {
	SchemaVersion     string          `json:"schema_version"`
	ImportanceScore   float64         `json:"importance_score"`
	MarketImpact      string          `json:"market_impact"`
	NoveltyScore      float64         `json:"novelty_score"`
	Confidence        float64         `json:"confidence"`
	SummaryVI         string          `json:"summary_vi"`
	SummaryEN         string          `json:"summary_en"`
	EventTitle        string          `json:"event_title"`
	EventType         string          `json:"event_type"`
	AffectedTickers   []string        `json:"affected_tickers"`
	AffectedCompanies []string        `json:"affected_companies"`
	AffectedSectors   []string        `json:"affected_sectors"`
	AffectedAssets    []string        `json:"affected_assets"`
	Countries         []string        `json:"countries"`
	KeyFacts          json.RawMessage `json:"key_facts"`
	NewInformation    json.RawMessage `json:"new_information"`
	RiskFlags         []string        `json:"risk_flags"`
	Sentiment         string          `json:"sentiment"`
	TimeSensitivity   string          `json:"time_sensitivity"`
	DedupEventKey     string          `json:"dedup_event_key"`
}

func ParseAnalysis(text string) (*db.Analysis, error) {
	obj, err := extractJSONObject(text)
	if err != nil {
		return nil, err
	}
	var raw rawAnalysis
	if err := json.Unmarshal([]byte(obj), &raw); err != nil {
		return nil, fmt.Errorf("parse analysis json: %w", err)
	}
	if raw.EventTitle == "" {
		return nil, errors.New("analysis missing event_title")
	}
	if raw.KeyFacts == nil {
		raw.KeyFacts = []byte("[]")
	}
	if raw.NewInformation == nil {
		raw.NewInformation = []byte("[]")
	}
	ana := &db.Analysis{ImportanceScore: clamp(raw.ImportanceScore), NoveltyScore: clamp(raw.NoveltyScore), Confidence: clamp(raw.Confidence), MarketImpact: enum(raw.MarketImpact, []string{"low", "medium", "high", "critical"}, "low"), Sentiment: enum(raw.Sentiment, []string{"bullish", "bearish", "neutral", "mixed"}, "neutral"), EventType: enum(raw.EventType, []string{"macro", "fed", "earnings", "guidance", "mna", "ipo", "sec_filing", "regulation", "lawsuit", "analyst_rating", "commodity", "crypto", "forex", "rates", "labor_market", "inflation", "geopolitical", "company_news", "market_move", "other"}, "other"), EventTitle: raw.EventTitle, DedupEventKey: strings.ToLower(raw.DedupEventKey), SummaryVI: raw.SummaryVI, SummaryEN: raw.SummaryEN, AffectedTickers: upper(raw.AffectedTickers), AffectedCompanies: raw.AffectedCompanies, AffectedSectors: raw.AffectedSectors, AffectedAssets: raw.AffectedAssets, Countries: raw.Countries, KeyFacts: raw.KeyFacts, NewInformation: raw.NewInformation, RiskFlags: raw.RiskFlags, TimeSensitivity: enum(raw.TimeSensitivity, []string{"immediate", "today", "this_week", "long_term"}, "today"), RawJSON: []byte(obj)}
	return ana, nil
}

func extractJSONObject(s string) (string, error) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return "", errors.New("no JSON object in model output")
	}
	return s[start : end+1], nil
}
func clamp(n float64) int {
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return int(n)
}
func enum(v string, allowed []string, def string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return def
}
func upper(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		x = strings.ToUpper(strings.TrimSpace(x))
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
func loadPrompt(path, def string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	return string(b)
}

const defaultSystemPrompt = "Return strict JSON only for a financial news article."
const defaultUserTemplate = "Title: {{title}}\nSource: {{source}}\nPublished: {{published_at}}\nExcerpt: {{excerpt}}\nContent: {{content}}"
