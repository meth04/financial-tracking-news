CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('rss','api','html')),
    url TEXT NOT NULL,
    credibility_score INT NOT NULL DEFAULT 70 CHECK (credibility_score BETWEEN 0 AND 100),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    full_content_allowed BOOLEAN NOT NULL DEFAULT FALSE,
    crawl_interval_minutes INT NOT NULL DEFAULT 10,
    max_age_hours INT NOT NULL DEFAULT 72,
    rate_limit_per_minute INT NOT NULL DEFAULT 30,
    respect_robots BOOLEAN NOT NULL DEFAULT TRUE,
    user_agent TEXT,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS source_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('running','success','partial','failed')),
    fetched_count INT NOT NULL DEFAULT 0,
    inserted_raw_count INT NOT NULL DEFAULT 0,
    inserted_article_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS raw_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    source_run_id UUID REFERENCES source_runs(id) ON DELETE SET NULL,
    raw_url TEXT NOT NULL,
    canonical_url TEXT,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    http_status INT,
    content_type TEXT,
    raw_hash TEXT NOT NULL,
    raw_payload BYTEA,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(source_id, raw_hash)
);

CREATE TABLE IF NOT EXISTS articles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    raw_item_id UUID REFERENCES raw_items(id) ON DELETE SET NULL,
    canonical_url TEXT,
    title TEXT NOT NULL,
    normalized_title TEXT NOT NULL,
    author TEXT,
    excerpt TEXT,
    content_text TEXT,
    content_html TEXT,
    language TEXT NOT NULL DEFAULT 'en',
    published_at TIMESTAMPTZ,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    time_confidence TEXT NOT NULL DEFAULT 'medium' CHECK (time_confidence IN ('high','medium','low')),
    status TEXT NOT NULL DEFAULT 'new',
    is_outdated BOOLEAN NOT NULL DEFAULT FALSE,
    title_hash TEXT NOT NULL,
    content_hash TEXT,
    simhash BIGINT,
    word_count INT NOT NULL DEFAULT 0,
    source_credibility_score INT NOT NULL DEFAULT 70,
    processing_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_canonical_url_unique
ON articles(canonical_url)
WHERE canonical_url IS NOT NULL AND canonical_url <> '';

CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_fetched_at ON articles(fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_status ON articles(status);
CREATE INDEX IF NOT EXISTS idx_articles_outdated ON articles(is_outdated);
CREATE INDEX IF NOT EXISTS idx_articles_title_hash ON articles(title_hash);
CREATE INDEX IF NOT EXISTS idx_articles_content_hash ON articles(content_hash);
CREATE INDEX IF NOT EXISTS idx_articles_simhash ON articles(simhash);
CREATE INDEX IF NOT EXISTS idx_articles_title_trgm ON articles USING gin (normalized_title gin_trgm_ops);

CREATE TABLE IF NOT EXISTS article_duplicates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    duplicate_of_article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    duplicate_type TEXT NOT NULL CHECK (duplicate_type IN ('url','title','content_hash','near_duplicate','manual')),
    similarity_score NUMERIC(5,4),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(article_id, duplicate_of_article_id)
);

CREATE TABLE IF NOT EXISTS llm_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    article_id UUID NOT NULL UNIQUE REFERENCES articles(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','done','failed')),
    priority INT NOT NULL DEFAULT 0,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    last_heartbeat_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_llm_jobs_pick
ON llm_jobs(status, next_run_at, priority DESC, created_at ASC);

CREATE TABLE IF NOT EXISTS article_llm_analysis (
    article_id UUID PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    importance_score INT NOT NULL CHECK (importance_score BETWEEN 0 AND 100),
    novelty_score INT NOT NULL CHECK (novelty_score BETWEEN 0 AND 100),
    confidence INT NOT NULL CHECK (confidence BETWEEN 0 AND 100),
    market_impact TEXT NOT NULL CHECK (market_impact IN ('low','medium','high','critical')),
    sentiment TEXT NOT NULL CHECK (sentiment IN ('bullish','bearish','neutral','mixed')),
    event_type TEXT NOT NULL,
    event_title TEXT NOT NULL,
    dedup_event_key TEXT,
    summary_vi TEXT,
    summary_en TEXT,
    affected_tickers TEXT[] NOT NULL DEFAULT '{}',
    affected_companies TEXT[] NOT NULL DEFAULT '{}',
    affected_sectors TEXT[] NOT NULL DEFAULT '{}',
    affected_assets TEXT[] NOT NULL DEFAULT '{}',
    countries TEXT[] NOT NULL DEFAULT '{}',
    key_facts JSONB NOT NULL DEFAULT '[]'::jsonb,
    new_information JSONB NOT NULL DEFAULT '[]'::jsonb,
    risk_flags TEXT[] NOT NULL DEFAULT '{}',
    time_sensitivity TEXT NOT NULL DEFAULT 'today',
    raw_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_analysis_importance ON article_llm_analysis(importance_score DESC);
CREATE INDEX IF NOT EXISTS idx_analysis_tickers ON article_llm_analysis USING gin(affected_tickers);
CREATE INDEX IF NOT EXISTS idx_analysis_event_type ON article_llm_analysis(event_type);
CREATE INDEX IF NOT EXISTS idx_analysis_event_key ON article_llm_analysis(dedup_event_key);

CREATE TABLE IF NOT EXISTS event_clusters (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_key TEXT NOT NULL,
    event_title TEXT NOT NULL,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','quiet','outdated','merged')),
    importance_score INT NOT NULL DEFAULT 0 CHECK (importance_score BETWEEN 0 AND 100),
    novelty_score INT NOT NULL DEFAULT 0 CHECK (novelty_score BETWEEN 0 AND 100),
    confidence INT NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 100),
    affected_tickers TEXT[] NOT NULL DEFAULT '{}',
    affected_sectors TEXT[] NOT NULL DEFAULT '{}',
    affected_assets TEXT[] NOT NULL DEFAULT '{}',
    source_count INT NOT NULL DEFAULT 0,
    article_count INT NOT NULL DEFAULT 0,
    update_count INT NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    summary_vi TEXT,
    summary_en TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_clusters_event_key ON event_clusters(event_key);
CREATE INDEX IF NOT EXISTS idx_clusters_last_updated ON event_clusters(last_updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_clusters_importance ON event_clusters(importance_score DESC);
CREATE INDEX IF NOT EXISTS idx_clusters_tickers ON event_clusters USING gin(affected_tickers);

CREATE TABLE IF NOT EXISTS event_articles (
    event_cluster_id UUID NOT NULL REFERENCES event_clusters(id) ON DELETE CASCADE,
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    relation TEXT NOT NULL CHECK (relation IN ('original','duplicate','related','update')),
    similarity_score NUMERIC(5,4),
    novelty_score INT CHECK (novelty_score BETWEEN 0 AND 100),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(event_cluster_id, article_id)
);

CREATE TABLE IF NOT EXISTS event_updates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_cluster_id UUID NOT NULL REFERENCES event_clusters(id) ON DELETE CASCADE,
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    update_summary TEXT NOT NULL,
    new_facts JSONB NOT NULL DEFAULT '[]'::jsonb,
    importance_delta INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_event_updates_cluster ON event_updates(event_cluster_id, created_at DESC);

CREATE TABLE IF NOT EXISTS app_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    level TEXT NOT NULL,
    component TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
