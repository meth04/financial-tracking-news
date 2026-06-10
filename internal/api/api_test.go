package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nguyen/financial-tracking-news/internal/config"
)

func TestHealthRouteWithoutStore(t *testing.T) {
	h := New(nil, config.Defaults(), nil, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}
func TestQueryParsingDefaults(t *testing.T) {
	if !parseBoolDefault("", true) {
		t.Fatal("fresh_only default")
	}
	if parseIntDefault("bad", 50) != 50 {
		t.Fatal("int default")
	}
	if clamp(500, 1, 200) != 200 {
		t.Fatal("clamp")
	}
}

func TestStaticFrontendServesIndexAndSPAFallback(t *testing.T) {
	if _, ok := staticDistDir(); !ok {
		t.Skip("web/dist not built")
	}
	h := New(nil, config.Defaults(), nil, nil)
	for _, path := range []string{"/", "/articles/detail"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != 200 {
			t.Fatalf("%s status %d", path, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "root") {
			t.Fatalf("%s did not serve frontend index", path)
		}
	}
}
