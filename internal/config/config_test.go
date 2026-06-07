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
	if len(srcs) == 0 {
		t.Fatal("expected seed sources")
	}
	for _, s := range srcs {
		if s.MaxAgeHours != 72 {
			t.Fatalf("source %s max age %d", s.Key, s.MaxAgeHours)
		}
	}
}
