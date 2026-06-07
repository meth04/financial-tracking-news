package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultAppConfigPath     = "config/app.example.yaml"
	DefaultSourcesConfigPath = "config/sources.seed.yaml"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	Freshness FreshnessConfig `yaml:"freshness"`
	Crawler   CrawlerConfig   `yaml:"crawler"`
	Storage   StorageConfig   `yaml:"storage"`
	LLM       LLMConfig       `yaml:"llm"`
	API       APIConfig       `yaml:"api"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type AppConfig struct {
	Name      string `yaml:"name"`
	Env       string `yaml:"env"`
	Port      int    `yaml:"port"`
	PublicURL string `yaml:"public_url"`
}

type FreshnessConfig struct {
	MaxAgeHours      int  `yaml:"max_age_hours"`
	DefaultFreshOnly bool `yaml:"default_fresh_only"`
}

type CrawlerConfig struct {
	DefaultIntervalMinutes int    `yaml:"default_interval_minutes"`
	RunOnStartup           bool   `yaml:"run_on_startup"`
	MaxParallelSources     int    `yaml:"max_parallel_sources"`
	HTTPTimeoutSeconds     int    `yaml:"http_timeout_seconds"`
	HTTPMaxBodyMB          int64  `yaml:"http_max_body_mb"`
	UserAgent              string `yaml:"user_agent"`
	RespectRobots          bool   `yaml:"respect_robots"`
}

type StorageConfig struct {
	DatabaseURLEnv       string `yaml:"database_url_env"`
	RawRetentionDays     int    `yaml:"raw_retention_days"`
	ArticleRetentionDays int    `yaml:"article_retention_days"`
	ClusterRetentionDays int    `yaml:"cluster_retention_days"`
}

type LLMConfig struct {
	BaseURL        string   `yaml:"base_url"`
	APIKeyEnv      string   `yaml:"api_key_env"`
	Model          string   `yaml:"model"`
	MaxConcurrency int      `yaml:"max_concurrency"`
	TimeoutSeconds int      `yaml:"timeout_seconds"`
	MaxAttempts    int      `yaml:"max_attempts"`
	RetryBackoff   []string `yaml:"retry_backoff"`
}

type APIConfig struct {
	PageSizeDefault    int      `yaml:"page_size_default"`
	PageSizeMax        int      `yaml:"page_size_max"`
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type SourceSeedFile struct {
	Sources []SourceConfig `yaml:"sources"`
}

type SourceConfig struct {
	Key                  string         `yaml:"key" json:"key"`
	Name                 string         `yaml:"name" json:"name"`
	Type                 string         `yaml:"type" json:"type"`
	URL                  string         `yaml:"url" json:"url"`
	CredibilityScore     int            `yaml:"credibility_score" json:"credibility_score"`
	Enabled              bool           `yaml:"enabled" json:"enabled"`
	FullContentAllowed   bool           `yaml:"full_content_allowed" json:"full_content_allowed"`
	CrawlIntervalMinutes int            `yaml:"crawl_interval_minutes" json:"crawl_interval_minutes"`
	MaxAgeHours          int            `yaml:"max_age_hours" json:"max_age_hours"`
	RateLimitPerMinute   int            `yaml:"rate_limit_per_minute" json:"rate_limit_per_minute"`
	RespectRobots        bool           `yaml:"respect_robots" json:"respect_robots"`
	Tags                 []string       `yaml:"tags" json:"tags"`
	Config               map[string]any `yaml:"config" json:"config"`
}

func Defaults() Config {
	return Config{
		App:       AppConfig{Name: "financial-news-intelligence", Env: "development", Port: 8080, PublicURL: "http://localhost:8080"},
		Freshness: FreshnessConfig{MaxAgeHours: 72, DefaultFreshOnly: true},
		Crawler:   CrawlerConfig{DefaultIntervalMinutes: 10, RunOnStartup: true, MaxParallelSources: 4, HTTPTimeoutSeconds: 25, HTTPMaxBodyMB: 8, UserAgent: "FinancialNewsIntel/0.1 contact:local@example.com", RespectRobots: true},
		Storage:   StorageConfig{DatabaseURLEnv: "DATABASE_URL", RawRetentionDays: 7, ArticleRetentionDays: 14, ClusterRetentionDays: 30},
		LLM:       LLMConfig{BaseURL: "http://localhost:8317/v1", APIKeyEnv: "LLM_API_KEY", Model: "gemini-3.1-flash-lite-preview", MaxConcurrency: 3, TimeoutSeconds: 90, MaxAttempts: 5, RetryBackoff: []string{"1m", "5m", "15m", "1h"}},
		API:       APIConfig{PageSizeDefault: 50, PageSizeMax: 200, CORSAllowedOrigins: []string{"http://localhost:5173", "http://localhost:8080"}},
		Logging:   LoggingConfig{Level: "info", Format: "text"},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		path = getenvDefault("APP_CONFIG", DefaultAppConfigPath)
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	applyEnv(&cfg)
	return cfg, cfg.Validate()
}

func LoadSources(path string) ([]SourceConfig, error) {
	if path == "" {
		path = DefaultSourcesConfigPath
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f SourceSeedFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	for i := range f.Sources {
		if f.Sources[i].Config == nil {
			f.Sources[i].Config = map[string]any{}
		}
	}
	return f.Sources, validateSources(f.Sources)
}

func (c Config) Validate() error {
	var errs []string
	if c.App.Port <= 0 || c.App.Port > 65535 {
		errs = append(errs, "app.port must be 1-65535")
	}
	if c.Freshness.MaxAgeHours <= 0 {
		errs = append(errs, "freshness.max_age_hours must be positive")
	}
	if c.Crawler.DefaultIntervalMinutes <= 0 {
		errs = append(errs, "crawler.default_interval_minutes must be positive")
	}
	if c.Storage.DatabaseURLEnv == "" {
		errs = append(errs, "storage.database_url_env is required")
	}
	if c.LLM.BaseURL == "" || c.LLM.Model == "" {
		errs = append(errs, "llm.base_url and llm.model are required")
	}
	if c.LLM.MaxConcurrency <= 0 {
		errs = append(errs, "llm.max_concurrency must be positive")
	}
	if c.API.PageSizeDefault <= 0 || c.API.PageSizeMax < c.API.PageSizeDefault {
		errs = append(errs, "api page size config invalid")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateSources(sources []SourceConfig) error {
	seen := map[string]bool{}
	var errs []string
	for _, s := range sources {
		if s.Key == "" || s.Name == "" || s.URL == "" {
			errs = append(errs, "source key/name/url are required")
		}
		if seen[s.Key] {
			errs = append(errs, "duplicate source key "+s.Key)
		}
		seen[s.Key] = true
		switch s.Type {
		case "rss", "api", "html":
		default:
			errs = append(errs, "unsupported source type "+s.Type)
		}
		if s.MaxAgeHours <= 0 {
			errs = append(errs, s.Key+": max_age_hours must be positive")
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (c Config) DatabaseURL() string {
	return os.Getenv(c.Storage.DatabaseURLEnv)
}

func (c Config) LLMAPIKey() string {
	key := os.Getenv(c.LLM.APIKeyEnv)
	if key == "" {
		return "sk-my-key-is-empty"
	}
	return key
}

func (c Config) FreshnessDuration() time.Duration {
	return time.Duration(c.Freshness.MaxAgeHours) * time.Hour
}

func (c Config) BackoffDurations() []time.Duration {
	out := make([]time.Duration, 0, len(c.LLM.RetryBackoff))
	for _, s := range c.LLM.RetryBackoff {
		if d, err := time.ParseDuration(s); err == nil {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		out = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	}
	return out
}

func applyEnv(c *Config) {
	if v := os.Getenv("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.App.Port = n
		}
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		c.LLM.BaseURL = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		c.LLM.Model = v
	}
	if v := os.Getenv("LLM_MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.LLM.MaxConcurrency = n
		}
	}
	if v := os.Getenv("FRESHNESS_MAX_AGE_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Freshness.MaxAgeHours = n
		}
	}
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
