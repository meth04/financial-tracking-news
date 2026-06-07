package cluster

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/dedup"
	"github.com/nguyen/financial-tracking-news/internal/normalize"
)

type Store interface {
	FindCandidateClusters(context.Context, string, string, []string, time.Time, string) ([]db.EventCluster, error)
	CreateCluster(context.Context, db.EventCluster) (uuid.UUID, error)
	AttachArticleToCluster(context.Context, uuid.UUID, uuid.UUID, string, float64, int, string) error
	InsertEventUpdate(context.Context, uuid.UUID, uuid.UUID, string, json.RawMessage, int) error
	RefreshClusterAggregates(context.Context, uuid.UUID) error
}

type Service struct {
	Store           Store
	FreshnessWindow time.Duration
}

func (s Service) ClusterArticle(ctx context.Context, article db.Article, ana db.Analysis) (uuid.UUID, string, error) {
	if s.FreshnessWindow == 0 {
		s.FreshnessWindow = 72 * time.Hour
	}
	key := ana.DedupEventKey
	if key == "" {
		key = buildKey(ana)
	}
	cands, err := s.Store.FindCandidateClusters(ctx, key, ana.EventType, ana.AffectedTickers, time.Now().Add(-s.FreshnessWindow), ana.EventTitle)
	if err != nil {
		return uuid.Nil, "", err
	}
	bestScore := 0
	var best *db.EventCluster
	for i := range cands {
		sc := score(cands[i], key, ana)
		if sc > bestScore {
			bestScore = sc
			best = &cands[i]
		}
	}
	if best == nil || bestScore < 50 {
		c := db.EventCluster{EventKey: key, EventTitle: firstNonEmpty(ana.EventTitle, article.Title), EventType: firstNonEmpty(ana.EventType, "other"), Status: "active", ImportanceScore: ana.ImportanceScore, NoveltyScore: ana.NoveltyScore, Confidence: ana.Confidence, AffectedTickers: ana.AffectedTickers, AffectedSectors: ana.AffectedSectors, AffectedAssets: ana.AffectedAssets, SourceCount: 1, ArticleCount: 1, LastSeenAt: article.FetchedAt, LastUpdatedAt: article.FetchedAt, SummaryVI: &ana.SummaryVI, SummaryEN: &ana.SummaryEN}
		id, err := s.Store.CreateCluster(ctx, c)
		if err != nil {
			return uuid.Nil, "", err
		}
		if err := s.Store.AttachArticleToCluster(ctx, id, article.ID, "original", 1, ana.NoveltyScore, "new event cluster"); err != nil {
			return uuid.Nil, "", err
		}
		return id, "original", s.Store.RefreshClusterAggregates(ctx, id)
	}
	hasNew := jsonArrayLen(ana.NewInformation) > 0
	relation := dedup.ClassifyUpdate(false, true, ana.NoveltyScore, hasNew, 0)
	if bestScore >= 70 && ana.NoveltyScore < 20 && !hasNew {
		relation = "duplicate"
	}
	reason := "same event score threshold"
	if err := s.Store.AttachArticleToCluster(ctx, best.ID, article.ID, relation, float64(bestScore)/100, ana.NoveltyScore, reason); err != nil {
		return uuid.Nil, "", err
	}
	if relation == "update" {
		summary := ana.SummaryEN
		if summary == "" {
			summary = article.Title
		}
		_ = s.Store.InsertEventUpdate(ctx, best.ID, article.ID, summary, ana.NewInformation, max(0, ana.ImportanceScore-best.ImportanceScore))
	}
	return best.ID, relation, s.Store.RefreshClusterAggregates(ctx, best.ID)
}

func score(c db.EventCluster, key string, a db.Analysis) int {
	sc := 0
	if key != "" && c.EventKey == key {
		sc += 40
	}
	if c.EventType == a.EventType {
		sc += 20
	}
	if overlap(c.AffectedTickers, a.AffectedTickers) {
		sc += 20
	}
	if overlap(c.AffectedSectors, a.AffectedSectors) {
		sc += 10
	}
	if normalize.Similarity(c.EventTitle, a.EventTitle) > 0.5 {
		sc += 20
	}
	return sc
}
func overlap(a, b []string) bool {
	set := map[string]bool{}
	for _, x := range a {
		set[strings.ToUpper(x)] = true
	}
	for _, y := range b {
		if set[strings.ToUpper(y)] {
			return true
		}
	}
	return false
}
func buildKey(a db.Analysis) string {
	ent := "general"
	if len(a.AffectedTickers) > 0 {
		ent = strings.ToLower(a.AffectedTickers[0])
	} else if len(a.AffectedAssets) > 0 {
		ent = strings.ToLower(a.AffectedAssets[0])
	}
	slug := strings.ToLower(a.EventTitle)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, slug)
	slug = strings.Trim(slug, "-")
	if len(slug) > 48 {
		slug = slug[:48]
	}
	return firstNonEmpty(a.EventType, "other") + ":" + ent + ":" + slug
}
func firstNonEmpty(v, def string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return def
}
func jsonArrayLen(b json.RawMessage) int {
	var arr []any
	if len(b) == 0 {
		return 0
	}
	_ = json.Unmarshal(b, &arr)
	return len(arr)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
