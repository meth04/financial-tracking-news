package federalreg

import (
	"strings"
	"testing"
	"time"
)

func TestBuildURL(t *testing.T) {
	u := BuildURL("https://www.federalregister.gov/api/v1/documents.json", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), []string{"sec"}, []string{"RULE"})
	if !strings.Contains(u, "conditions%5Bpublication_date%5D%5Bgte%5D=2026-06-01") || !strings.Contains(u, "conditions%5Bagencies%5D%5B%5D=sec") {
		t.Fatalf("unexpected url %s", u)
	}
}
