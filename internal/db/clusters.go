package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Store) FindCandidateClusters(ctx context.Context, eventKey, eventType string, tickers []string, since time.Time, title string) ([]EventCluster, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,event_key,event_title,event_type,status,importance_score,novelty_score,confidence,affected_tickers,affected_sectors,affected_assets,source_count,article_count,update_count,first_seen_at,last_seen_at,last_updated_at,summary_vi,summary_en,created_at,updated_at
FROM event_clusters WHERE last_seen_at >= ? AND (event_key=? OR event_type=? OR affected_tickers LIKE ? OR lower(event_title) LIKE '%' || lower(?) || '%') ORDER BY last_updated_at DESC LIMIT 50`, since, eventKey, eventType, "%"+first(tickers)+"%", title)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EventCluster{}
	for rows.Next() {
		c, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCluster(ctx context.Context, c EventCluster) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO event_clusters(id,event_key,event_title,event_type,status,importance_score,novelty_score,confidence,affected_tickers,affected_sectors,affected_assets,source_count,article_count,update_count,last_seen_at,last_updated_at,summary_vi,summary_en)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id.String(), c.EventKey, c.EventTitle, c.EventType, coalesce(c.Status, "active"), c.ImportanceScore, c.NoveltyScore, c.Confidence, jsonText(c.AffectedTickers), jsonText(c.AffectedSectors), jsonText(c.AffectedAssets), c.SourceCount, c.ArticleCount, c.UpdateCount, c.LastSeenAt, c.LastUpdatedAt, strPtr(c.SummaryVI), strPtr(c.SummaryEN))
	return id, err
}

func (s *Store) AttachArticleToCluster(ctx context.Context, clusterID, articleID uuid.UUID, relation string, score float64, novelty int, reason string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO event_articles(event_cluster_id,article_id,relation,similarity_score,novelty_score,reason) VALUES(?,?,?,?,?,?) ON CONFLICT(event_cluster_id,article_id) DO UPDATE SET relation=excluded.relation,similarity_score=excluded.similarity_score,novelty_score=excluded.novelty_score,reason=excluded.reason`, clusterID.String(), articleID.String(), relation, score, novelty, reason)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE articles SET status='clustered', updated_at=CURRENT_TIMESTAMP WHERE id=?`, articleID.String())
	return err
}

func (s *Store) InsertEventUpdate(ctx context.Context, clusterID, articleID uuid.UUID, summary string, facts json.RawMessage, delta int) error {
	if len(facts) == 0 {
		facts = []byte("[]")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO event_updates(id,event_cluster_id,article_id,update_summary,new_facts,importance_delta) VALUES(?,?,?,?,?,?)`, uuid.NewString(), clusterID.String(), articleID.String(), summary, string(facts), delta)
	return err
}

func (s *Store) RefreshClusterAggregates(ctx context.Context, clusterID uuid.UUID) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE event_clusters SET
article_count=(SELECT count(*) FROM event_articles WHERE event_cluster_id=?),
update_count=(SELECT count(*) FROM event_articles WHERE event_cluster_id=? AND relation='update'),
source_count=(SELECT count(DISTINCT art.source_id) FROM event_articles ea JOIN articles art ON art.id=ea.article_id WHERE ea.event_cluster_id=?),
importance_score=MAX(importance_score, COALESCE((SELECT max(importance_score) FROM event_articles ea JOIN article_llm_analysis ana ON ana.article_id=ea.article_id WHERE ea.event_cluster_id=?),0)),
novelty_score=MAX(novelty_score, COALESCE((SELECT max(novelty_score) FROM event_articles ea JOIN article_llm_analysis ana ON ana.article_id=ea.article_id WHERE ea.event_cluster_id=?),0)),
last_seen_at=COALESCE((SELECT max(art.fetched_at) FROM event_articles ea JOIN articles art ON art.id=ea.article_id WHERE ea.event_cluster_id=?), last_seen_at),
last_updated_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`, clusterID.String(), clusterID.String(), clusterID.String(), clusterID.String(), clusterID.String(), clusterID.String(), clusterID.String())
	return err
}

func (s *Store) ListClusters(ctx context.Context, f ClusterFilters) (ListResult[EventCluster], error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 50
	}
	where, args := clusterWhere(f)
	q := `SELECT id,event_key,event_title,event_type,status,importance_score,novelty_score,confidence,affected_tickers,affected_sectors,affected_assets,source_count,article_count,update_count,first_seen_at,last_seen_at,last_updated_at,summary_vi,summary_en,created_at,updated_at FROM event_clusters cl ` + where + ` ORDER BY ` + safeClusterOrder(f.Sort, f.Order) + ` LIMIT ? OFFSET ?`
	rows, err := s.DB.QueryContext(ctx, q, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return ListResult[EventCluster]{}, err
	}
	defer rows.Close()
	items := []EventCluster{}
	for rows.Next() {
		c, err := scanCluster(rows)
		if err != nil {
			return ListResult[EventCluster]{}, err
		}
		items = append(items, c)
	}
	var total int
	_ = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM event_clusters cl `+where, args...).Scan(&total)
	return ListResult[EventCluster]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func clusterWhere(f ClusterFilters) (string, []any) {
	parts := []string{"1=1"}
	args := []any{}
	add := func(expr string, v any) { args = append(args, v); parts = append(parts, expr) }
	if f.FreshOnly {
		parts = append(parts, "cl.status <> 'outdated' AND cl.last_seen_at >= datetime('now','-72 hours')")
	}
	if f.Q != "" {
		add("(lower(cl.event_title) LIKE '%' || lower(?) || '%' OR lower(COALESCE(cl.summary_en,'')) LIKE '%' || lower(?) || '%')", f.Q)
		args = append(args, f.Q)
	}
	if f.Ticker != "" {
		add("upper(cl.affected_tickers) LIKE '%' || upper(?) || '%'", f.Ticker)
	}
	if f.EventType != "" {
		add("cl.event_type=?", f.EventType)
	}
	if f.ImpactMin > 0 {
		add("cl.importance_score >= ?", f.ImpactMin)
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func safeClusterOrder(sort, order string) string {
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	switch sort {
	case "importance":
		return "importance_score " + dir
	case "article_count":
		return "article_count " + dir
	case "update_count":
		return "update_count " + dir
	default:
		return "last_updated_at " + dir
	}
}

func (s *Store) GetCluster(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	cluster, err := scanCluster(s.DB.QueryRowContext(ctx, `SELECT id,event_key,event_title,event_type,status,importance_score,novelty_score,confidence,affected_tickers,affected_sectors,affected_assets,source_count,article_count,update_count,first_seen_at,last_seen_at,last_updated_at,summary_vi,summary_en,created_at,updated_at FROM event_clusters WHERE id=?`, id.String()))
	if err != nil {
		return nil, err
	}
	out := map[string]any{"cluster": cluster}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+articleSelectColumns()+`, ea.relation FROM event_articles ea JOIN articles art ON art.id=ea.article_id JOIN sources src ON src.id=art.source_id WHERE ea.event_cluster_id=? ORDER BY art.fetched_at DESC`, id.String())
	if err == nil {
		defer rows.Close()
		articles := []map[string]any{}
		for rows.Next() {
			a, rel, e := scanClusterArticle(rows)
			if e == nil {
				articles = append(articles, map[string]any{"article": a, "relation": rel})
			}
		}
		out["articles"] = articles
	}
	urows, err := s.DB.QueryContext(ctx, `SELECT id,event_cluster_id,article_id,update_summary,new_facts,importance_delta,created_at FROM event_updates WHERE event_cluster_id=? ORDER BY created_at DESC`, id.String())
	if err == nil {
		defer urows.Close()
		updates := []EventUpdate{}
		for urows.Next() {
			var u EventUpdate
			var idS, clS, artS, facts string
			_ = urows.Scan(&idS, &clS, &artS, &u.UpdateSummary, &facts, &u.ImportanceDelta, &u.CreatedAt)
			u.ID = uuid.MustParse(idS)
			u.EventClusterID = uuid.MustParse(clS)
			u.ArticleID = uuid.MustParse(artS)
			u.NewFacts = []byte(facts)
			updates = append(updates, u)
		}
		out["updates"] = updates
	}
	return out, nil
}

func scanCluster(row scanner) (EventCluster, error) {
	var c EventCluster
	var id string
	var svi, sen sql.NullString
	var tickers, sectors, assets string
	if err := row.Scan(&id, &c.EventKey, &c.EventTitle, &c.EventType, &c.Status, &c.ImportanceScore, &c.NoveltyScore, &c.Confidence, &tickers, &sectors, &assets, &c.SourceCount, &c.ArticleCount, &c.UpdateCount, &c.FirstSeenAt, &c.LastSeenAt, &c.LastUpdatedAt, &svi, &sen, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return EventCluster{}, err
	}
	c.ID = uuid.MustParse(id)
	c.AffectedTickers = parseStringArray(tickers)
	c.AffectedSectors = parseStringArray(sectors)
	c.AffectedAssets = parseStringArray(assets)
	if svi.Valid {
		c.SummaryVI = &svi.String
	}
	if sen.Valid {
		c.SummaryEN = &sen.String
	}
	return c, nil
}

func scanClusterArticle(row scanner) (Article, string, error) {
	var a Article
	var id, srcID string
	var rawID, canon, author, excerpt, txt, html, chash, perr sql.NullString
	var pub sql.NullTime
	var sim sql.NullInt64
	var outdated int
	var rel string
	if err := row.Scan(&id, &srcID, &a.SourceKey, &a.SourceName, &rawID, &canon, &a.Title, &a.NormalizedTitle, &author, &excerpt, &txt, &html, &a.Language, &pub, &a.FetchedAt, &a.TimeConfidence, &a.Status, &outdated, &a.TitleHash, &chash, &sim, &a.WordCount, &a.SourceCredibilityScore, &perr, &a.CreatedAt, &a.UpdatedAt, &rel); err != nil {
		return Article{}, "", err
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
	return a, rel, nil
}

func (s *Store) ListSourceHealth(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT src.key,src.name,src.type,src.enabled,src.credibility_score,src.crawl_interval_minutes, sr.status,sr.started_at,sr.finished_at,COALESCE(sr.fetched_count,0),COALESCE(sr.inserted_raw_count,0),COALESCE(sr.inserted_article_count,0),sr.error_message,COALESCE(sr.metadata,'{}'),(SELECT count(*) FROM source_runs r WHERE r.source_id=src.id AND r.status='failed') FROM sources src LEFT JOIN source_runs sr ON sr.id=(SELECT id FROM source_runs r WHERE r.source_id=src.id ORDER BY r.started_at DESC LIMIT 1) ORDER BY src.key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var key, name, typ string
		var enabled int
		var cred, interval, fetched, raw, articles, errs int
		var status, msg sql.NullString
		var metaText string
		var started, finished sql.NullTime
		if err := rows.Scan(&key, &name, &typ, &enabled, &cred, &interval, &status, &started, &finished, &fetched, &raw, &articles, &msg, &metaText, &errs); err != nil {
			return nil, err
		}
		metadata := map[string]any{}
		_ = json.Unmarshal([]byte(metaText), &metadata)
		m := map[string]any{"key": key, "name": name, "type": typ, "enabled": enabled != 0, "credibility_score": cred, "crawl_interval_minutes": interval, "error_count": errs, "health": "unknown", "last_fetched_count": fetched, "last_inserted_raw_count": raw, "last_inserted_article_count": articles, "last_run_metadata": metadata}
		if reason, ok := metadata["no_fresh_reason"].(string); ok && reason != "" {
			m["last_no_fresh_reason"] = reason
		}
		for _, key := range []string{"candidate_links", "raw_candidate_count", "older_than_window_count", "quality_rejected_count", "content_fetch_failed", "missing_date_count", "returned_fresh_count", "article_inserted_count"} {
			if v, ok := metadata[key]; ok {
				m["last_"+key] = v
			}
		}
		if status.Valid {
			m["last_status"] = status.String
			switch status.String {
			case "success":
				m["health"] = "ok"
			case "running":
				m["health"] = "running"
			case "partial":
				m["health"] = "warning"
			default:
				m["health"] = "error"
			}
		}
		if started.Valid {
			m["last_started_at"] = started.Time
		}
		if finished.Valid {
			m["last_finished_at"] = finished.Time
		}
		if msg.Valid {
			m["last_error"] = msg.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) LLMQueueStatus(ctx context.Context) (map[string]any, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT status,count(*) FROM llm_jobs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{"pending": 0, "running": 0, "done": 0, "failed": 0}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		counts[st] = n
	}
	jobs := []LLMJob{}
	jrows, err := s.DB.QueryContext(ctx, `SELECT id,article_id,status,priority,attempts,max_attempts,next_run_at,locked_at,locked_by,last_heartbeat_at,last_error,created_at,updated_at FROM llm_jobs ORDER BY updated_at DESC LIMIT 100`)
	if err == nil {
		defer jrows.Close()
		for jrows.Next() {
			j, e := scanJob(jrows)
			if e == nil {
				jobs = append(jobs, j)
			}
		}
	}
	return map[string]any{"counts": counts, "jobs": jobs}, nil
}

func coalesce(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func first(xs []string) string {
	if len(xs) > 0 {
		return xs[0]
	}
	return ""
}
