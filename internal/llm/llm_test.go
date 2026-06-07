package llm

import "testing"

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
