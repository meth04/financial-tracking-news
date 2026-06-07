package cluster

import (
	"encoding/json"
	"testing"

	"github.com/nguyen/financial-tracking-news/internal/db"
)

func TestEventScoreMatching(t *testing.T) {
	c := db.EventCluster{EventKey: "fed:rates:hold", EventType: "fed", EventTitle: "Fed holds rates", AffectedTickers: []string{"SPY"}}
	a := db.Analysis{DedupEventKey: "fed:rates:hold", EventType: "fed", EventTitle: "Federal Reserve holds rates", AffectedTickers: []string{"SPY"}}
	if score(c, a.DedupEventKey, a) < 70 {
		t.Fatal("expected same event score")
	}
}
func TestJSONArrayLen(t *testing.T) {
	b := json.RawMessage(`["new CPI number"]`)
	if jsonArrayLen(b) != 1 {
		t.Fatal("expected len 1")
	}
}
