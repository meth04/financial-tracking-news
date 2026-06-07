package db

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Source struct {
	ID                   uuid.UUID      `json:"id"`
	Key                  string         `json:"key"`
	Name                 string         `json:"name"`
	Type                 string         `json:"type"`
	URL                  string         `json:"url"`
	CredibilityScore     int            `json:"credibility_score"`
	Enabled              bool           `json:"enabled"`
	FullContentAllowed   bool           `json:"full_content_allowed"`
	CrawlIntervalMinutes int            `json:"crawl_interval_minutes"`
	MaxAgeHours          int            `json:"max_age_hours"`
	RateLimitPerMinute   int            `json:"rate_limit_per_minute"`
	RespectRobots        bool           `json:"respect_robots"`
	UserAgent            *string        `json:"user_agent,omitempty"`
	Config               map[string]any `json:"config"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type SourceRun struct {
	ID                   uuid.UUID      `json:"id"`
	SourceID             uuid.UUID      `json:"source_id"`
	StartedAt            time.Time      `json:"started_at"`
	FinishedAt           *time.Time     `json:"finished_at,omitempty"`
	Status               string         `json:"status"`
	FetchedCount         int            `json:"fetched_count"`
	InsertedRawCount     int            `json:"inserted_raw_count"`
	InsertedArticleCount int            `json:"inserted_article_count"`
	ErrorMessage         *string        `json:"error_message,omitempty"`
	Metadata             map[string]any `json:"metadata"`
}

type RawItem struct {
	ID           uuid.UUID      `json:"id"`
	SourceID     uuid.UUID      `json:"source_id"`
	SourceRunID  *uuid.UUID     `json:"source_run_id,omitempty"`
	RawURL       string         `json:"raw_url"`
	CanonicalURL *string        `json:"canonical_url,omitempty"`
	FetchedAt    time.Time      `json:"fetched_at"`
	PublishedAt  *time.Time     `json:"published_at,omitempty"`
	HTTPStatus   *int           `json:"http_status,omitempty"`
	ContentType  *string        `json:"content_type,omitempty"`
	RawHash      string         `json:"raw_hash"`
	RawPayload   []byte         `json:"-"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Article struct {
	ID                     uuid.UUID       `json:"id"`
	SourceID               uuid.UUID       `json:"source_id"`
	SourceKey              string          `json:"source_key,omitempty"`
	SourceName             string          `json:"source_name,omitempty"`
	RawItemID              *uuid.UUID      `json:"raw_item_id,omitempty"`
	CanonicalURL           *string         `json:"canonical_url,omitempty"`
	Title                  string          `json:"title"`
	NormalizedTitle        string          `json:"normalized_title"`
	Author                 *string         `json:"author,omitempty"`
	Excerpt                *string         `json:"excerpt,omitempty"`
	ContentText            *string         `json:"content_text,omitempty"`
	ContentHTML            *string         `json:"content_html,omitempty"`
	Language               string          `json:"language"`
	PublishedAt            *time.Time      `json:"published_at,omitempty"`
	FetchedAt              time.Time       `json:"fetched_at"`
	TimeConfidence         string          `json:"time_confidence"`
	Status                 string          `json:"status"`
	IsOutdated             bool            `json:"is_outdated"`
	TitleHash              string          `json:"title_hash"`
	ContentHash            *string         `json:"content_hash,omitempty"`
	Simhash                *int64          `json:"simhash,omitempty"`
	WordCount              int             `json:"word_count"`
	SourceCredibilityScore int             `json:"source_credibility_score"`
	ProcessingError        *string         `json:"processing_error,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	Analysis               *Analysis       `json:"analysis,omitempty"`
	Cluster                *ClusterSummary `json:"cluster,omitempty"`
	Duplicate              *DuplicateInfo  `json:"duplicate,omitempty"`
}

type DuplicateInfo struct {
	DuplicateOfArticleID uuid.UUID `json:"duplicate_of_article_id"`
	DuplicateType        string    `json:"duplicate_type"`
	SimilarityScore      *float64  `json:"similarity_score,omitempty"`
	Reason               *string   `json:"reason,omitempty"`
}

type ArticleDuplicate struct {
	ID                   uuid.UUID `json:"id"`
	ArticleID            uuid.UUID `json:"article_id"`
	DuplicateOfArticleID uuid.UUID `json:"duplicate_of_article_id"`
	DuplicateType        string    `json:"duplicate_type"`
	SimilarityScore      *float64  `json:"similarity_score,omitempty"`
	Reason               *string   `json:"reason,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

type LLMJob struct {
	ID              uuid.UUID  `json:"id"`
	ArticleID       uuid.UUID  `json:"article_id"`
	Status          string     `json:"status"`
	Priority        int        `json:"priority"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"max_attempts"`
	NextRunAt       time.Time  `json:"next_run_at"`
	LockedAt        *time.Time `json:"locked_at,omitempty"`
	LockedBy        *string    `json:"locked_by,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	LastError       *string    `json:"last_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Analysis struct {
	ArticleID         uuid.UUID       `json:"article_id"`
	Model             string          `json:"model"`
	ImportanceScore   int             `json:"importance_score"`
	NoveltyScore      int             `json:"novelty_score"`
	Confidence        int             `json:"confidence"`
	MarketImpact      string          `json:"market_impact"`
	Sentiment         string          `json:"sentiment"`
	EventType         string          `json:"event_type"`
	EventTitle        string          `json:"event_title"`
	DedupEventKey     string          `json:"dedup_event_key"`
	SummaryVI         string          `json:"summary_vi"`
	SummaryEN         string          `json:"summary_en"`
	AffectedTickers   []string        `json:"affected_tickers"`
	AffectedCompanies []string        `json:"affected_companies"`
	AffectedSectors   []string        `json:"affected_sectors"`
	AffectedAssets    []string        `json:"affected_assets"`
	Countries         []string        `json:"countries"`
	KeyFacts          json.RawMessage `json:"key_facts"`
	NewInformation    json.RawMessage `json:"new_information"`
	RiskFlags         []string        `json:"risk_flags"`
	TimeSensitivity   string          `json:"time_sensitivity"`
	RawJSON           json.RawMessage `json:"raw_json"`
	CreatedAt         time.Time       `json:"created_at"`
}

type EventCluster struct {
	ID              uuid.UUID `json:"id"`
	EventKey        string    `json:"event_key"`
	EventTitle      string    `json:"event_title"`
	EventType       string    `json:"event_type"`
	Status          string    `json:"status"`
	ImportanceScore int       `json:"importance_score"`
	NoveltyScore    int       `json:"novelty_score"`
	Confidence      int       `json:"confidence"`
	AffectedTickers []string  `json:"affected_tickers"`
	AffectedSectors []string  `json:"affected_sectors"`
	AffectedAssets  []string  `json:"affected_assets"`
	SourceCount     int       `json:"source_count"`
	ArticleCount    int       `json:"article_count"`
	UpdateCount     int       `json:"update_count"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	LastUpdatedAt   time.Time `json:"last_updated_at"`
	SummaryVI       *string   `json:"summary_vi,omitempty"`
	SummaryEN       *string   `json:"summary_en,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ClusterSummary struct {
	ID          uuid.UUID `json:"id"`
	EventTitle  string    `json:"event_title"`
	EventType   string    `json:"event_type"`
	Relation    string    `json:"relation"`
	UpdateCount int       `json:"update_count"`
}

type EventArticle struct {
	EventClusterID  uuid.UUID `json:"event_cluster_id"`
	ArticleID       uuid.UUID `json:"article_id"`
	Relation        string    `json:"relation"`
	SimilarityScore *float64  `json:"similarity_score,omitempty"`
	NoveltyScore    *int      `json:"novelty_score,omitempty"`
	Reason          *string   `json:"reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	Article         *Article  `json:"article,omitempty"`
}

type EventUpdate struct {
	ID              uuid.UUID       `json:"id"`
	EventClusterID  uuid.UUID       `json:"event_cluster_id"`
	ArticleID       uuid.UUID       `json:"article_id"`
	UpdateSummary   string          `json:"update_summary"`
	NewFacts        json.RawMessage `json:"new_facts"`
	ImportanceDelta int             `json:"importance_delta"`
	CreatedAt       time.Time       `json:"created_at"`
}

type ArticleFilters struct {
	Q         string
	Source    string
	Ticker    string
	EventType string
	Impact    string
	Sentiment string
	Status    string
	FreshOnly bool
	Page      int
	PageSize  int
	Sort      string
	Order     string
}

type ClusterFilters struct {
	Q         string
	Ticker    string
	EventType string
	ImpactMin int
	FreshOnly bool
	Page      int
	PageSize  int
	Sort      string
	Order     string
}

type ListResult[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}
