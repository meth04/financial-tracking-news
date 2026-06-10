package config

import "testing"

func TestDefaultsValidate(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestInvalidConfig(t *testing.T) {
	c := Defaults()
	c.Freshness.MaxAgeHours = 0
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
func TestLoadSourcesSeed(t *testing.T) {
	srcs, err := LoadSources("../../config/sources.seed.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) < 29 {
		t.Fatalf("expected original, RSS, and HTML source additions, got %d", len(srcs))
	}
	seen := map[string]bool{}
	added := map[string]bool{
		"fed_testimony": true, "fed_press_bcreg": true, "fed_press_enforcement": true, "sec_press_releases": true,
		"fdic_press_releases": true, "cftc_press_releases": true, "cftc_enforcement_press_releases": true, "cfpb_newsroom": true,
		"eia_today_in_energy": true, "eia_press_releases": true, "ftc_business_blog": true, "nyfed_liberty_street_economics": true,
		"stlouisfed_on_the_economy": true, "fred_blog": true, "atlantafed_macroblog": true,
		"fed_press_releases_html": true, "sec_press_releases_html": true, "fdic_press_releases_html": true,
		"bea_current_releases_html": true, "cfpb_newsroom_html": true, "frb_services_news_html": true,
	}
	for _, s := range srcs {
		if s.MaxAgeHours != 72 {
			t.Fatalf("source %s max age %d", s.Key, s.MaxAgeHours)
		}
		if seen[s.Key] {
			t.Fatalf("duplicate source key %s", s.Key)
		}
		seen[s.Key] = true
		if s.Type == "html" && s.FullContentAllowed {
			if s.Config == nil || s.Config["html"] == nil {
				t.Fatalf("html source %s missing selector config", s.Key)
			}
			if s.Config["min_content_chars"] == nil || s.Config["min_word_count"] == nil || s.Config["require_article_content"] == nil {
				t.Fatalf("html source %s missing quality config", s.Key)
			}
		}
		delete(added, s.Key)
	}
	if len(added) > 0 {
		t.Fatalf("missing added source keys: %#v", added)
	}
}
