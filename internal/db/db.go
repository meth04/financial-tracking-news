package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/config"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Connect(ctx context.Context, cfg config.Config) (*Store, error) {
	path := os.Getenv("SQLITE_PATH")
	if path == "" {
		path = "finnews.db"
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() {
	if s != nil && s.DB != nil {
		_ = s.DB.Close()
	}
}

func (s *Store) Migrate(ctx context.Context, _ string) error {
	_, err := s.DB.ExecContext(ctx, sqliteSchema)
	return err
}

func (s *Store) SeedSources(ctx context.Context, sources []config.SourceConfig) error {
	for _, src := range sources {
		cfg, _ := json.Marshal(src.Config)
		_, err := s.DB.ExecContext(ctx, `INSERT INTO sources(id,key,name,type,url,credibility_score,enabled,full_content_allowed,crawl_interval_minutes,max_age_hours,rate_limit_per_minute,respect_robots,config,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
ON CONFLICT(key) DO UPDATE SET name=excluded.name,type=excluded.type,url=excluded.url,credibility_score=excluded.credibility_score,enabled=excluded.enabled,full_content_allowed=excluded.full_content_allowed,crawl_interval_minutes=excluded.crawl_interval_minutes,max_age_hours=excluded.max_age_hours,rate_limit_per_minute=excluded.rate_limit_per_minute,respect_robots=excluded.respect_robots,config=excluded.config,updated_at=CURRENT_TIMESTAMP`,
			uuid.NewString(), src.Key, src.Name, src.Type, src.URL, src.CredibilityScore, boolInt(src.Enabled), boolInt(src.FullContentAllowed), src.CrawlIntervalMinutes, src.MaxAgeHours, src.RateLimitPerMinute, boolInt(src.RespectRobots), string(cfg))
		if err != nil {
			return fmt.Errorf("seed %s: %w", src.Key, err)
		}
	}
	return nil
}

func (s *Store) ListEnabledSources(ctx context.Context) ([]Source, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,key,name,type,url,credibility_score,enabled,full_content_allowed,crawl_interval_minutes,max_age_hours,rate_limit_per_minute,respect_robots,user_agent,config,created_at,updated_at FROM sources WHERE enabled=1 ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) GetSourceByKey(ctx context.Context, key string) (Source, error) {
	return scanSource(s.DB.QueryRowContext(ctx, `SELECT id,key,name,type,url,credibility_score,enabled,full_content_allowed,crawl_interval_minutes,max_age_hours,rate_limit_per_minute,respect_robots,user_agent,config,created_at,updated_at FROM sources WHERE key=?`, key))
}

func scanSource(row scanner) (Source, error) {
	var src Source
	var id, cfg string
	var ua sql.NullString
	var enabled, full, robots int
	if err := row.Scan(&id, &src.Key, &src.Name, &src.Type, &src.URL, &src.CredibilityScore, &enabled, &full, &src.CrawlIntervalMinutes, &src.MaxAgeHours, &src.RateLimitPerMinute, &robots, &ua, &cfg, &src.CreatedAt, &src.UpdatedAt); err != nil {
		return Source{}, err
	}
	src.ID = uuid.MustParse(id)
	src.Enabled = enabled != 0
	src.FullContentAllowed = full != 0
	src.RespectRobots = robots != 0
	if ua.Valid {
		src.UserAgent = &ua.String
	}
	_ = json.Unmarshal([]byte(cfg), &src.Config)
	if src.Config == nil {
		src.Config = map[string]any{}
	}
	return src, nil
}

func (s *Store) CreateSourceRun(ctx context.Context, sourceID uuid.UUID) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO source_runs(id,source_id,status) VALUES(?,?,'running')`, id.String(), sourceID.String())
	return id, err
}

func (s *Store) FinishSourceRun(ctx context.Context, id uuid.UUID, status string, fetched, raw, articles int, runErr error) error {
	var msg any
	if runErr != nil {
		msg = runErr.Error()
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE source_runs SET finished_at=CURRENT_TIMESTAMP,status=?,fetched_count=?,inserted_raw_count=?,inserted_article_count=?,error_message=? WHERE id=?`, status, fetched, raw, articles, msg, id.String())
	return err
}

func (s *Store) InsertRawItem(ctx context.Context, item RawItem) (uuid.UUID, bool, error) {
	id := uuid.New()
	meta, _ := json.Marshal(item.Metadata)
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO raw_items(id,source_id,source_run_id,raw_url,canonical_url,fetched_at,published_at,http_status,content_type,raw_hash,raw_payload,metadata) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		id.String(), item.SourceID.String(), uuidPtrString(item.SourceRunID), item.RawURL, strPtr(item.CanonicalURL), item.FetchedAt, timePtr(item.PublishedAt), intPtr(item.HTTPStatus), strPtr(item.ContentType), item.RawHash, item.RawPayload, string(meta))
	if err != nil {
		return uuid.Nil, false, err
	}
	var got string
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM raw_items WHERE source_id=? AND raw_hash=?`, item.SourceID.String(), item.RawHash).Scan(&got); err != nil {
		return uuid.Nil, false, err
	}
	return uuid.MustParse(got), got == id.String(), nil
}

func (s *Store) InsertArticle(ctx context.Context, a Article) (uuid.UUID, bool, error) {
	id := uuid.New()
	res, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO articles(id,source_id,raw_item_id,canonical_url,title,normalized_title,author,excerpt,content_text,content_html,language,published_at,fetched_at,time_confidence,status,is_outdated,title_hash,content_hash,simhash,word_count,source_credibility_score,processing_error)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id.String(), a.SourceID.String(), uuidPtrString(a.RawItemID), strPtr(a.CanonicalURL), a.Title, a.NormalizedTitle, strPtr(a.Author), strPtr(a.Excerpt), strPtr(a.ContentText), strPtr(a.ContentHTML), a.Language, timePtr(a.PublishedAt), a.FetchedAt, a.TimeConfidence, a.Status, boolInt(a.IsOutdated), a.TitleHash, strPtr(a.ContentHash), int64Ptr(a.Simhash), a.WordCount, a.SourceCredibilityScore, strPtr(a.ProcessingError))
	if err != nil {
		return uuid.Nil, false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return id, true, nil
	}
	if a.CanonicalURL != nil && *a.CanonicalURL != "" {
		var got string
		if err := s.DB.QueryRowContext(ctx, `SELECT id FROM articles WHERE canonical_url=?`, *a.CanonicalURL).Scan(&got); err != nil {
			return uuid.Nil, false, err
		}
		return uuid.MustParse(got), false, nil
	}
	return uuid.Nil, false, sql.ErrNoRows
}

func (s *Store) UpdateArticleStatus(ctx context.Context, articleID uuid.UUID, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE articles SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, articleID.String())
	return err
}

func (s *Store) FindExactDuplicate(ctx context.Context, a Article, window time.Duration) (*Article, string, error) {
	cutoff := time.Now().Add(-window)
	if a.CanonicalURL != nil && *a.CanonicalURL != "" {
		if art, ok, err := s.findOneArticle(ctx, `art.canonical_url=?`, *a.CanonicalURL); err != nil || ok {
			return art, "url", err
		}
	}
	if a.ContentHash != nil && *a.ContentHash != "" {
		if art, ok, err := s.findOneArticle(ctx, `art.content_hash=?`, *a.ContentHash); err != nil || ok {
			return art, "content_hash", err
		}
	}
	if art, ok, err := s.findOneArticle(ctx, `art.source_id=? AND art.title_hash=? AND COALESCE(art.published_at, art.fetched_at) >= ?`, a.SourceID.String(), a.TitleHash, cutoff); err != nil || ok {
		return art, "title", err
	}
	return nil, "", nil
}

func (s *Store) findOneArticle(ctx context.Context, where string, args ...any) (*Article, bool, error) {
	q := `SELECT ` + articleSelectColumns() + ` FROM articles art JOIN sources src ON src.id=art.source_id WHERE ` + where + ` ORDER BY art.created_at ASC LIMIT 1`
	row := s.DB.QueryRowContext(ctx, q, args...)
	art, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &art, true, nil
}

func (s *Store) InsertDuplicate(ctx context.Context, articleID, duplicateOf uuid.UUID, dupType string, score *float64, reason string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO article_duplicates(id,article_id,duplicate_of_article_id,duplicate_type,similarity_score,reason) VALUES(?,?,?,?,?,?)`, uuid.NewString(), articleID.String(), duplicateOf.String(), dupType, floatPtr(score), reason)
	return err
}

func (s *Store) EnqueueLLMJob(ctx context.Context, articleID uuid.UUID, priority, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO llm_jobs(id,article_id,status,priority,max_attempts,next_run_at) SELECT ?,?,'pending',?,?,CURRENT_TIMESTAMP WHERE NOT EXISTS (SELECT 1 FROM article_llm_analysis WHERE article_id=?)`, uuid.NewString(), articleID.String(), priority, maxAttempts, articleID.String())
	return err
}

func (s *Store) PickLLMJob(ctx context.Context, worker string) (*LLMJob, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `SELECT id,article_id,status,priority,attempts,max_attempts,next_run_at,locked_at,locked_by,last_heartbeat_at,last_error,created_at,updated_at FROM llm_jobs WHERE status='pending' AND next_run_at <= CURRENT_TIMESTAMP ORDER BY priority DESC, created_at ASC LIMIT 1`)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE llm_jobs SET status='running',locked_at=CURRENT_TIMESTAMP,locked_by=?,last_heartbeat_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=?`, worker, job.ID.String())
	if err != nil {
		return nil, err
	}
	job.Status = "running"
	return &job, tx.Commit()
}

func (s *Store) CompleteLLMJob(ctx context.Context, jobID uuid.UUID) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE llm_jobs SET status='done', updated_at=CURRENT_TIMESTAMP WHERE id=?`, jobID.String())
	return err
}

func (s *Store) FailLLMJob(ctx context.Context, jobID uuid.UUID, errText string, backoff time.Duration) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE llm_jobs SET attempts=attempts+1,status=CASE WHEN attempts+1 >= max_attempts THEN 'failed' ELSE 'pending' END,next_run_at=?,last_error=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, time.Now().Add(backoff), errText, jobID.String())
	return err
}

func (s *Store) RetryFailedLLM(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE llm_jobs SET status='pending',next_run_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE status='failed'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) HasAnalysis(ctx context.Context, articleID uuid.UUID) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM article_llm_analysis WHERE article_id=?`, articleID.String()).Scan(&n)
	return n > 0, err
}

func (s *Store) SaveAnalysis(ctx context.Context, a Analysis) error {
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO article_llm_analysis(article_id,model,importance_score,novelty_score,confidence,market_impact,sentiment,event_type,event_title,dedup_event_key,summary_vi,summary_en,affected_tickers,affected_companies,affected_sectors,affected_assets,countries,key_facts,new_information,risk_flags,time_sensitivity,raw_json)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ArticleID.String(), a.Model, a.ImportanceScore, a.NoveltyScore, a.Confidence, a.MarketImpact, a.Sentiment, a.EventType, a.EventTitle, a.DedupEventKey, a.SummaryVI, a.SummaryEN, jsonText(a.AffectedTickers), jsonText(a.AffectedCompanies), jsonText(a.AffectedSectors), jsonText(a.AffectedAssets), jsonText(a.Countries), string(defaultJSON(a.KeyFacts, "[]")), string(defaultJSON(a.NewInformation, "[]")), jsonText(a.RiskFlags), a.TimeSensitivity, string(defaultJSON(a.RawJSON, "{}")))
	return err
}

func (s *Store) GetArticle(ctx context.Context, id uuid.UUID) (Article, error) {
	q := `SELECT ` + articleSelectColumns() + `, ` + analysisSelectColumns() + `, cl.id,cl.event_title,cl.event_type,ea.relation,cl.update_count, dup.duplicate_of_article_id,dup.duplicate_type,dup.similarity_score,dup.reason FROM articles art JOIN sources src ON src.id=art.source_id LEFT JOIN article_llm_analysis ana ON ana.article_id=art.id LEFT JOIN event_articles ea ON ea.article_id=art.id LEFT JOIN event_clusters cl ON cl.id=ea.event_cluster_id LEFT JOIN article_duplicates dup ON dup.article_id=art.id WHERE art.id=?`
	return scanArticleWithRelations(s.DB.QueryRowContext(ctx, q, id.String()))
}

func (s *Store) ListArticles(ctx context.Context, f ArticleFilters) (ListResult[Article], error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 50
	}
	where, args := articleWhere(f)
	order := safeArticleOrder(f.Sort, f.Order)
	q := `SELECT ` + articleSelectColumns() + `, ` + analysisSelectColumns() + `, cl.id,cl.event_title,cl.event_type,ea.relation,cl.update_count, dup.duplicate_of_article_id,dup.duplicate_type,dup.similarity_score,dup.reason FROM articles art JOIN sources src ON src.id=art.source_id LEFT JOIN article_llm_analysis ana ON ana.article_id=art.id LEFT JOIN event_articles ea ON ea.article_id=art.id LEFT JOIN event_clusters cl ON cl.id=ea.event_cluster_id LEFT JOIN article_duplicates dup ON dup.article_id=art.id ` + where + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	rows, err := s.DB.QueryContext(ctx, q, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return ListResult[Article]{}, err
	}
	defer rows.Close()
	items := []Article{}
	for rows.Next() {
		a, err := scanArticleWithRelations(rows)
		if err != nil {
			return ListResult[Article]{}, err
		}
		items = append(items, a)
	}
	var total int
	_ = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM articles art JOIN sources src ON src.id=art.source_id LEFT JOIN article_llm_analysis ana ON ana.article_id=art.id `+where, args...).Scan(&total)
	return ListResult[Article]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func articleWhere(f ArticleFilters) (string, []any) {
	parts := []string{"1=1"}
	args := []any{}
	add := func(expr string, v any) { args = append(args, v); parts = append(parts, expr) }
	if f.FreshOnly {
		parts = append(parts, "art.is_outdated=0")
	}
	if f.Q != "" {
		add("(lower(art.title) LIKE '%' || lower(?) || '%' OR lower(COALESCE(art.excerpt,'')) LIKE '%' || lower(?) || '%' OR lower(COALESCE(ana.summary_en,'')) LIKE '%' || lower(?) || '%')", f.Q)
		args = append(args, f.Q, f.Q)
	}
	if f.Source != "" {
		add("src.key=?", f.Source)
	}
	if f.Ticker != "" {
		add("upper(COALESCE(ana.affected_tickers,'')) LIKE '%' || upper(?) || '%'", f.Ticker)
	}
	if f.EventType != "" {
		add("ana.event_type=?", f.EventType)
	}
	if f.Impact != "" {
		add("ana.market_impact=?", f.Impact)
	}
	if f.Sentiment != "" {
		add("ana.sentiment=?", f.Sentiment)
	}
	if f.Status != "" {
		add("art.status=?", f.Status)
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func safeArticleOrder(sort, order string) string {
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	switch sort {
	case "importance":
		return "ana.importance_score " + dir + ", COALESCE(art.published_at, art.fetched_at) DESC"
	case "source":
		return "src.key " + dir
	default:
		return "COALESCE(art.published_at, art.fetched_at) " + dir
	}
}

func (s *Store) Stats(ctx context.Context) (map[string]any, error) {
	stats := map[string]any{}
	queries := map[string]string{
		"fresh_articles": `SELECT count(*) FROM articles WHERE is_outdated=0`,
		"active_events":  `SELECT count(*) FROM event_clusters WHERE status='active'`,
		"high_impact":    `SELECT count(*) FROM article_llm_analysis ana JOIN articles art ON art.id=ana.article_id WHERE art.is_outdated=0 AND ana.market_impact IN ('high','critical')`,
		"llm_pending":    `SELECT count(*) FROM llm_jobs WHERE status IN ('pending','running')`,
		"source_errors":  `SELECT count(*) FROM source_runs WHERE status='failed'`,
	}
	for k, q := range queries {
		var n int
		if err := s.DB.QueryRowContext(ctx, q).Scan(&n); err != nil {
			return nil, err
		}
		stats[k] = n
	}
	var newest sql.NullTime
	_ = s.DB.QueryRowContext(ctx, `SELECT max(fetched_at) FROM raw_items`).Scan(&newest)
	if newest.Valid {
		stats["newest_fetched_at"] = newest.Time
	}
	return stats, nil
}

func (s *Store) Health(ctx context.Context) map[string]any {
	out := map[string]any{"status": "ok", "database": "sqlite", "checked_at": time.Now().UTC()}
	if err := s.DB.PingContext(ctx); err != nil {
		out["status"] = "degraded"
		out["database"] = "error"
		out["error"] = err.Error()
	}
	return out
}

func articleSelectColumns() string {
	return `art.id,art.source_id,src.key,src.name,art.raw_item_id,art.canonical_url,art.title,art.normalized_title,art.author,art.excerpt,art.content_text,art.content_html,art.language,art.published_at,art.fetched_at,art.time_confidence,art.status,art.is_outdated,art.title_hash,art.content_hash,art.simhash,art.word_count,art.source_credibility_score,art.processing_error,art.created_at,art.updated_at`
}

func analysisSelectColumns() string {
	return `ana.article_id,COALESCE(ana.model,''),COALESCE(ana.importance_score,0),COALESCE(ana.novelty_score,0),COALESCE(ana.confidence,0),COALESCE(ana.market_impact,''),COALESCE(ana.sentiment,''),COALESCE(ana.event_type,''),COALESCE(ana.event_title,''),ana.dedup_event_key,ana.summary_vi,ana.summary_en,COALESCE(ana.affected_tickers,'[]'),COALESCE(ana.affected_companies,'[]'),COALESCE(ana.affected_sectors,'[]'),COALESCE(ana.affected_assets,'[]'),COALESCE(ana.countries,'[]'),COALESCE(ana.key_facts,'[]'),COALESCE(ana.new_information,'[]'),COALESCE(ana.risk_flags,'[]'),COALESCE(ana.time_sensitivity,''),COALESCE(ana.raw_json,'{}'),ana.created_at`
}

func scanArticle(row scanner) (Article, error) {
	var a Article
	var id, srcID string
	var rawID, canon, author, excerpt, txt, html, chash, perr sql.NullString
	var pub sql.NullTime
	var sim sql.NullInt64
	var outdated int
	if err := row.Scan(&id, &srcID, &a.SourceKey, &a.SourceName, &rawID, &canon, &a.Title, &a.NormalizedTitle, &author, &excerpt, &txt, &html, &a.Language, &pub, &a.FetchedAt, &a.TimeConfidence, &a.Status, &outdated, &a.TitleHash, &chash, &sim, &a.WordCount, &a.SourceCredibilityScore, &perr, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return Article{}, err
	}
	a.ID = uuid.MustParse(id)
	a.SourceID = uuid.MustParse(srcID)
	a.IsOutdated = outdated != 0
	if rawID.Valid {
		v := uuid.MustParse(rawID.String)
		a.RawItemID = &v
	}
	if canon.Valid {
		a.CanonicalURL = &canon.String
	}
	if author.Valid {
		a.Author = &author.String
	}
	if excerpt.Valid {
		a.Excerpt = &excerpt.String
	}
	if txt.Valid {
		a.ContentText = &txt.String
	}
	if html.Valid {
		a.ContentHTML = &html.String
	}
	if pub.Valid {
		a.PublishedAt = &pub.Time
	}
	if chash.Valid {
		a.ContentHash = &chash.String
	}
	if sim.Valid {
		v := sim.Int64
		a.Simhash = &v
	}
	if perr.Valid {
		a.ProcessingError = &perr.String
	}
	return a, nil
}

func scanArticleWithRelations(row scanner) (Article, error) {
	var a Article
	var id, srcID string
	var rawID, canon, author, excerpt, txt, html, chash, perr sql.NullString
	var pub sql.NullTime
	var sim sql.NullInt64
	var outdated int
	var ana Analysis
	var anaID, dedup, svi, sen, anaCreated sql.NullString
	var at, ac, as, aa, countries, keyFacts, newInfo, flags, rawJSON string
	var clID, clTitle, clType, rel sql.NullString
	var upd sql.NullInt64
	var dupID, dupType, dupReason sql.NullString
	var dupScore sql.NullFloat64
	if err := row.Scan(&id, &srcID, &a.SourceKey, &a.SourceName, &rawID, &canon, &a.Title, &a.NormalizedTitle, &author, &excerpt, &txt, &html, &a.Language, &pub, &a.FetchedAt, &a.TimeConfidence, &a.Status, &outdated, &a.TitleHash, &chash, &sim, &a.WordCount, &a.SourceCredibilityScore, &perr, &a.CreatedAt, &a.UpdatedAt,
		&anaID, &ana.Model, &ana.ImportanceScore, &ana.NoveltyScore, &ana.Confidence, &ana.MarketImpact, &ana.Sentiment, &ana.EventType, &ana.EventTitle, &dedup, &svi, &sen, &at, &ac, &as, &aa, &countries, &keyFacts, &newInfo, &flags, &ana.TimeSensitivity, &rawJSON, &anaCreated,
		&clID, &clTitle, &clType, &rel, &upd, &dupID, &dupType, &dupScore, &dupReason); err != nil {
		return Article{}, err
	}
	a.ID = uuid.MustParse(id)
	a.SourceID = uuid.MustParse(srcID)
	a.IsOutdated = outdated != 0
	if rawID.Valid {
		v := uuid.MustParse(rawID.String)
		a.RawItemID = &v
	}
	if canon.Valid {
		a.CanonicalURL = &canon.String
	}
	if author.Valid {
		a.Author = &author.String
	}
	if excerpt.Valid {
		a.Excerpt = &excerpt.String
	}
	if txt.Valid {
		a.ContentText = &txt.String
	}
	if html.Valid {
		a.ContentHTML = &html.String
	}
	if pub.Valid {
		a.PublishedAt = &pub.Time
	}
	if chash.Valid {
		a.ContentHash = &chash.String
	}
	if sim.Valid {
		v := sim.Int64
		a.Simhash = &v
	}
	if perr.Valid {
		a.ProcessingError = &perr.String
	}
	if anaID.Valid {
		ana.ArticleID = uuid.MustParse(anaID.String)
		ana.DedupEventKey = dedup.String
		ana.SummaryVI = svi.String
		ana.SummaryEN = sen.String
		ana.AffectedTickers = parseStringArray(at)
		ana.AffectedCompanies = parseStringArray(ac)
		ana.AffectedSectors = parseStringArray(as)
		ana.AffectedAssets = parseStringArray(aa)
		ana.Countries = parseStringArray(countries)
		ana.KeyFacts = []byte(keyFacts)
		ana.NewInformation = []byte(newInfo)
		ana.RiskFlags = parseStringArray(flags)
		ana.RawJSON = []byte(rawJSON)
		a.Analysis = &ana
	}
	if clID.Valid {
		a.Cluster = &ClusterSummary{ID: uuid.MustParse(clID.String), EventTitle: clTitle.String, EventType: clType.String, Relation: rel.String, UpdateCount: int(upd.Int64)}
	}
	if dupID.Valid {
		var sc *float64
		if dupScore.Valid {
			sc = &dupScore.Float64
		}
		var r *string
		if dupReason.Valid {
			r = &dupReason.String
		}
		a.Duplicate = &DuplicateInfo{DuplicateOfArticleID: uuid.MustParse(dupID.String), DuplicateType: dupType.String, SimilarityScore: sc, Reason: r}
	}
	return a, nil
}

func scanJob(row scanner) (LLMJob, error) {
	var j LLMJob
	var id, articleID string
	if err := row.Scan(&id, &articleID, &j.Status, &j.Priority, &j.Attempts, &j.MaxAttempts, &j.NextRunAt, &j.LockedAt, &j.LockedBy, &j.LastHeartbeatAt, &j.LastError, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return LLMJob{}, err
	}
	j.ID = uuid.MustParse(id)
	j.ArticleID = uuid.MustParse(articleID)
	return j, nil
}

type scanner interface{ Scan(dest ...any) error }

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func uuidPtrString(v *uuid.UUID) any {
	if v == nil {
		return nil
	}
	return v.String()
}
func strPtr(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
func timePtr(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
func intPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}
func int64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
func floatPtr(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
func jsonText(v any) string { b, _ := json.Marshal(v); return string(b) }
func parseStringArray(s string) []string {
	var out []string
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
func defaultJSON(b json.RawMessage, def string) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(def)
	}
	return b
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS sources (id TEXT PRIMARY KEY,key TEXT NOT NULL UNIQUE,name TEXT NOT NULL,type TEXT NOT NULL,url TEXT NOT NULL,credibility_score INTEGER NOT NULL DEFAULT 70,enabled INTEGER NOT NULL DEFAULT 1,full_content_allowed INTEGER NOT NULL DEFAULT 0,crawl_interval_minutes INTEGER NOT NULL DEFAULT 10,max_age_hours INTEGER NOT NULL DEFAULT 72,rate_limit_per_minute INTEGER NOT NULL DEFAULT 30,respect_robots INTEGER NOT NULL DEFAULT 1,user_agent TEXT,config TEXT NOT NULL DEFAULT '{}',created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS source_runs (id TEXT PRIMARY KEY,source_id TEXT NOT NULL,started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,finished_at DATETIME,status TEXT NOT NULL,fetched_count INTEGER NOT NULL DEFAULT 0,inserted_raw_count INTEGER NOT NULL DEFAULT 0,inserted_article_count INTEGER NOT NULL DEFAULT 0,error_message TEXT,metadata TEXT NOT NULL DEFAULT '{}',FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS raw_items (id TEXT PRIMARY KEY,source_id TEXT NOT NULL,source_run_id TEXT,raw_url TEXT NOT NULL,canonical_url TEXT,fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,published_at DATETIME,http_status INTEGER,content_type TEXT,raw_hash TEXT NOT NULL,raw_payload BLOB,metadata TEXT NOT NULL DEFAULT '{}',created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,UNIQUE(source_id, raw_hash));
CREATE TABLE IF NOT EXISTS articles (id TEXT PRIMARY KEY,source_id TEXT NOT NULL,raw_item_id TEXT,canonical_url TEXT UNIQUE,title TEXT NOT NULL,normalized_title TEXT NOT NULL,author TEXT,excerpt TEXT,content_text TEXT,content_html TEXT,language TEXT NOT NULL DEFAULT 'en',published_at DATETIME,fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,time_confidence TEXT NOT NULL DEFAULT 'medium',status TEXT NOT NULL DEFAULT 'new',is_outdated INTEGER NOT NULL DEFAULT 0,title_hash TEXT NOT NULL,content_hash TEXT,simhash INTEGER,word_count INTEGER NOT NULL DEFAULT 0,source_credibility_score INTEGER NOT NULL DEFAULT 70,processing_error TEXT,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE INDEX IF NOT EXISTS idx_articles_fetched ON articles(fetched_at DESC); CREATE INDEX IF NOT EXISTS idx_articles_status ON articles(status); CREATE INDEX IF NOT EXISTS idx_articles_outdated ON articles(is_outdated);
CREATE TABLE IF NOT EXISTS article_duplicates (id TEXT PRIMARY KEY,article_id TEXT NOT NULL,duplicate_of_article_id TEXT NOT NULL,duplicate_type TEXT NOT NULL,similarity_score REAL,reason TEXT,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,UNIQUE(article_id, duplicate_of_article_id));
CREATE TABLE IF NOT EXISTS llm_jobs (id TEXT PRIMARY KEY,article_id TEXT NOT NULL UNIQUE,status TEXT NOT NULL DEFAULT 'pending',priority INTEGER NOT NULL DEFAULT 0,attempts INTEGER NOT NULL DEFAULT 0,max_attempts INTEGER NOT NULL DEFAULT 5,next_run_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,locked_at DATETIME,locked_by TEXT,last_heartbeat_at DATETIME,last_error TEXT,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE INDEX IF NOT EXISTS idx_llm_jobs_pick ON llm_jobs(status,next_run_at,priority,created_at);
CREATE TABLE IF NOT EXISTS article_llm_analysis (article_id TEXT PRIMARY KEY,model TEXT NOT NULL,importance_score INTEGER NOT NULL,novelty_score INTEGER NOT NULL,confidence INTEGER NOT NULL,market_impact TEXT NOT NULL,sentiment TEXT NOT NULL,event_type TEXT NOT NULL,event_title TEXT NOT NULL,dedup_event_key TEXT,summary_vi TEXT,summary_en TEXT,affected_tickers TEXT NOT NULL DEFAULT '[]',affected_companies TEXT NOT NULL DEFAULT '[]',affected_sectors TEXT NOT NULL DEFAULT '[]',affected_assets TEXT NOT NULL DEFAULT '[]',countries TEXT NOT NULL DEFAULT '[]',key_facts TEXT NOT NULL DEFAULT '[]',new_information TEXT NOT NULL DEFAULT '[]',risk_flags TEXT NOT NULL DEFAULT '[]',time_sensitivity TEXT NOT NULL DEFAULT 'today',raw_json TEXT NOT NULL,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS event_clusters (id TEXT PRIMARY KEY,event_key TEXT NOT NULL,event_title TEXT NOT NULL,event_type TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'active',importance_score INTEGER NOT NULL DEFAULT 0,novelty_score INTEGER NOT NULL DEFAULT 0,confidence INTEGER NOT NULL DEFAULT 0,affected_tickers TEXT NOT NULL DEFAULT '[]',affected_sectors TEXT NOT NULL DEFAULT '[]',affected_assets TEXT NOT NULL DEFAULT '[]',source_count INTEGER NOT NULL DEFAULT 0,article_count INTEGER NOT NULL DEFAULT 0,update_count INTEGER NOT NULL DEFAULT 0,first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,last_updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,summary_vi TEXT,summary_en TEXT,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS event_articles (event_cluster_id TEXT NOT NULL,article_id TEXT NOT NULL,relation TEXT NOT NULL,similarity_score REAL,novelty_score INTEGER,reason TEXT,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(event_cluster_id, article_id));
CREATE TABLE IF NOT EXISTS event_updates (id TEXT PRIMARY KEY,event_cluster_id TEXT NOT NULL,article_id TEXT NOT NULL,update_summary TEXT NOT NULL,new_facts TEXT NOT NULL DEFAULT '[]',importance_delta INTEGER NOT NULL DEFAULT 0,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS app_logs (id TEXT PRIMARY KEY,level TEXT NOT NULL,component TEXT NOT NULL,message TEXT NOT NULL,metadata TEXT NOT NULL DEFAULT '{}',created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
`
