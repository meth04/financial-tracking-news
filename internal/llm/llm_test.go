package llm

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

func TestParseValidJSON(t *testing.T) {
	raw := `{"schema_version":"1.0","importance_score":120,"market_impact":"HIGH","novelty_score":-4,"confidence":77,"summary_vi":"vi","summary_en":"en","event_title":"Fed decision","event_type":"fed","affected_tickers":["spy"],"affected_companies":[],"affected_sectors":[],"affected_assets":[],"countries":["US"],"key_facts":["fact"],"new_information":[],"risk_flags":[],"sentiment":"neutral","time_sensitivity":"today","dedup_event_key":"Fed:Rates:Decision"}`
	a, err := ParseAnalysis(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.ImportanceScore != 100 || a.NoveltyScore != 0 || a.MarketImpact != "high" || a.AffectedTickers[0] != "SPY" {
		t.Fatalf("bad normalization %#v", a)
	}
}
func TestParseFencedJSON(t *testing.T) {
	raw := "```json\n{\"importance_score\":1,\"market_impact\":\"low\",\"novelty_score\":2,\"confidence\":3,\"event_title\":\"Event\",\"event_type\":\"other\",\"sentiment\":\"neutral\"}\n```"
	if _, err := ParseAnalysis(raw); err != nil {
		t.Fatal(err)
	}
}
func TestParseMalformedJSON(t *testing.T) {
	if _, err := ParseAnalysis("not json"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderPromptResolvesArticleTemplatePlaceholders(t *testing.T) {
	published := time.Date(2026, 6, 8, 9, 10, 11, 0, time.UTC)
	fetched := time.Date(2026, 6, 8, 9, 12, 13, 0, time.UTC)
	canonical := "https://example.com/news?clean=1"
	excerpt := "Short excerpt"
	content := "Full article content for the LLM prompt."
	author := "Reporter"
	article := db.Article{
		ID:                     uuid.New(),
		SourceName:             "Federal Reserve",
		SourceKey:              "fed_press_all",
		SourceCredibilityScore: 100,
		CanonicalURL:           &canonical,
		Title:                  "Fed leaves rates unchanged",
		NormalizedTitle:        "fed leaves rates unchanged",
		Author:                 &author,
		Excerpt:                &excerpt,
		ContentText:            &content,
		PublishedAt:            &published,
		FetchedAt:              fetched,
		TimeConfidence:         "high",
		Status:                 "llm_pending",
		WordCount:              7,
	}
	template := strings.Join([]string{
		"Source: {{source_name}} / {{source}} / {{source_key}}",
		"Credibility: {{source_credibility_score}}",
		"Published: {{published_at}}",
		"Fetched: {{fetched_at}}",
		"URL: {{canonical_url}}",
		"Title: {{title}}",
		"Normalized: {{normalized_title}}",
		"Author: {{author}}",
		"Time confidence: {{time_confidence}}",
		"Status: {{status}}",
		"Words: {{word_count}}",
		"Excerpt: {{excerpt}}",
		"Content: {{content_text}} / {{content}}",
	}, "\n")

	prompt := RenderPrompt(template, article)
	if strings.Contains(prompt, "{{") || strings.Contains(prompt, "}}") {
		t.Fatalf("unresolved template token in prompt:\n%s", prompt)
	}
	for _, want := range []string{"Federal Reserve", "fed_press_all", "100", canonical, article.Title, excerpt, content, published.Format(time.RFC3339), fetched.Format(time.RFC3339)} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
