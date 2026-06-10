package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/config"
	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/dedup"
	"github.com/nguyen/financial-tracking-news/internal/normalize"
	"github.com/nguyen/financial-tracking-news/internal/source"
	"github.com/nguyen/financial-tracking-news/internal/source/federalreg"
	htmlsrc "github.com/nguyen/financial-tracking-news/internal/source/html"
	"github.com/nguyen/financial-tracking-news/internal/source/rss"
	"github.com/nguyen/financial-tracking-news/internal/source/sec"
)

type Service struct {
	Store  *db.Store
	Config config.Config
	Log    *slog.Logger
	Client *http.Client
}

func New(store *db.Store, cfg config.Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{Store: store, Config: cfg, Log: log, Client: &http.Client{Timeout: time.Duration(cfg.Crawler.HTTPTimeoutSeconds) * time.Second}}
}

func (s *Service) StartScheduler(ctx context.Context) {
	interval := time.Duration(s.Config.Crawler.DefaultIntervalMinutes) * time.Minute
	if s.Config.Crawler.RunOnStartup {
		go func() { _ = s.CrawlOnce(ctx, "") }()
	}
	t := time.NewTicker(interval)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = s.CrawlOnce(ctx, "")
			}
		}
	}()
}

func (s *Service) CrawlOnce(ctx context.Context, onlySource string) error {
	sources, err := s.Store.ListEnabledSources(ctx)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, max(1, s.Config.Crawler.MaxParallelSources))
	errCh := make(chan error, len(sources))
	for _, src := range sources {
		if onlySource != "" && src.Key != onlySource {
			continue
		}
		src := src
		sem <- struct{}{}
		go func() { defer func() { <-sem }(); errCh <- s.crawlSource(ctx, src) }()
	}
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(errCh)
	for e := range errCh {
		if e != nil {
			s.Log.Warn("source crawl failed", "error", e)
		}
	}
	return nil
}

func (s *Service) crawlSource(ctx context.Context, src db.Source) error {
	runID, err := s.Store.CreateSourceRun(ctx, src.ID)
	if err != nil {
		return err
	}
	ad := s.adapter(src)
	if ad == nil {
		err := s.Store.FinishSourceRun(ctx, runID, "failed", 0, 0, 0, errUnsupported(src.Type))
		return err
	}
	since := time.Now().Add(-time.Duration(src.MaxAgeHours) * time.Hour)
	items, metadata, fetchErr := fetchWithMetadata(ctx, ad, since)
	metadata["source_key"] = src.Key
	metadata["since"] = since.UTC()
	insertedRaw := 0
	insertedArticles := 0
	status := "success"
	if fetchErr != nil {
		status = "failed"
	} else {
		for _, item := range items {
			res := s.processItem(ctx, src, runID, item)
			if res.RawInserted {
				insertedRaw++
			}
			if res.ArticleInserted {
				insertedArticles++
			}
			if res.QualityRejected {
				incMeta(metadata, "quality_rejected_count")
				if res.Reason != "" {
					metadata["last_quality_rejection_reason"] = res.Reason
				}
			}
			if res.ExactDuplicate {
				incMeta(metadata, "duplicate_count")
			}
			if res.Outdated {
				incMeta(metadata, "outdated_after_normalization_count")
			}
			if res.LLMEnqueued {
				incMeta(metadata, "llm_enqueued_count")
			}
		}
	}
	metadata["returned_item_count"] = len(items)
	metadata["raw_inserted_count"] = insertedRaw
	metadata["article_inserted_count"] = insertedArticles
	if insertedArticles == 0 && metadata["no_fresh_reason"] == nil {
		metadata["no_fresh_reason"] = noArticleReason(metadata, len(items))
	}
	if fetchErr != nil {
		s.Log.Warn("source fetch failed", "source", src.Key, "error", fetchErr)
	}
	return s.Store.FinishSourceRunWithMetadata(ctx, runID, status, len(items), insertedRaw, insertedArticles, fetchErr, metadata)
}

type processResult struct {
	RawInserted     bool
	ArticleInserted bool
	QualityRejected bool
	ExactDuplicate  bool
	Outdated        bool
	LLMEnqueued     bool
	Reason          string
}

func fetchWithMetadata(ctx context.Context, ad source.Adapter, since time.Time) ([]source.FetchedItem, map[string]any, error) {
	if diag, ok := ad.(source.DiagnosticAdapter); ok {
		res, err := diag.FetchWithDiagnostics(ctx, since)
		if res.Metadata == nil {
			res.Metadata = map[string]any{}
		}
		return res.Items, res.Metadata, err
	}
	items, err := ad.Fetch(ctx, since)
	return items, map[string]any{"adapter": ad.Name(), "returned_fresh_count": len(items)}, err
}

func (s *Service) processItem(ctx context.Context, src db.Source, runID uuid.UUID, item source.FetchedItem) processResult {
	// Raw item is persisted before normalization, deduplication, and LLM enqueueing.
	can := item.CanonicalURL
	if can == "" {
		can = normalize.CanonicalURL(item.RawURL)
	}
	payload := item.RawPayload
	if len(payload) == 0 {
		payload = source.RawPayload(item)
	}
	rh := normalize.RawHash(payload)
	ri := db.RawItem{SourceID: src.ID, SourceRunID: &runID, RawURL: item.RawURL, FetchedAt: item.FetchedAt, PublishedAt: item.PublishedAt, RawHash: rh, RawPayload: payload, Metadata: item.Metadata}
	if can != "" {
		ri.CanonicalURL = &can
	}
	if item.HTTPStatus != 0 {
		st := item.HTTPStatus
		ri.HTTPStatus = &st
	}
	if item.ContentType != "" {
		ct := item.ContentType
		ri.ContentType = &ct
	}
	rawID, rawInserted, err := s.Store.InsertRawItem(ctx, ri)
	if err != nil {
		s.Log.Warn("insert raw failed", "source", src.Key, "error", err)
		return processResult{Reason: err.Error()}
	}
	quality := assessItemQuality(src, item)
	if !quality.OK {
		s.Log.Warn("skip low-quality fetched item", "source", src.Key, "title", item.Title, "reason", quality.Reason, "content_chars", quality.ContentChars)
		return processResult{RawInserted: rawInserted, QualityRejected: true, Reason: quality.Reason}
	}
	article := normalizeItem(src, rawID, item, can, time.Duration(src.MaxAgeHours)*time.Hour)
	engine := dedup.Engine{Store: s.Store, Window: time.Duration(src.MaxAgeHours) * time.Hour}
	decision, err := engine.Decide(ctx, article)
	if err != nil {
		s.Log.Warn("dedup failed", "source", src.Key, "error", err)
	}
	articleID, inserted, err := s.Store.InsertArticle(ctx, article)
	if err != nil {
		s.Log.Warn("insert article failed", "source", src.Key, "error", err)
		return processResult{RawInserted: rawInserted, Reason: err.Error()}
	}
	res := processResult{RawInserted: rawInserted, ArticleInserted: inserted, Outdated: article.IsOutdated}
	if decision.DuplicateOf != nil && decision.Kind == dedup.KindExactDuplicate {
		_ = s.Store.InsertDuplicate(ctx, articleID, *decision.DuplicateOf, "content_hash", &decision.Similarity, decision.Reason)
		_ = s.Store.UpdateArticleStatus(ctx, articleID, "duplicate")
		res.ExactDuplicate = true
		res.Reason = decision.Reason
		return res
	}
	if !article.IsOutdated && decision.Kind != dedup.KindExactDuplicate {
		_ = s.Store.EnqueueLLMJob(ctx, articleID, 0, s.Config.LLM.MaxAttempts)
		_ = s.Store.UpdateArticleStatus(ctx, articleID, "llm_pending")
		res.LLMEnqueued = true
	}
	return res
}

type qualityDecision struct {
	OK           bool
	Reason       string
	ContentChars int
	WordCount    int
}

func assessItemQuality(src db.Source, item source.FetchedItem) qualityDecision {
	title := strings.TrimSpace(item.Title)
	text := normalize.CleanText(firstNonEmpty(item.ContentText, item.Excerpt))
	decision := qualityDecision{OK: true, ContentChars: len([]rune(text)), WordCount: normalize.WordCount(text)}
	if title == "" {
		decision.OK = false
		decision.Reason = "missing title"
		return decision
	}
	if looksLikeNavigationTitle(title) {
		decision.OK = false
		decision.Reason = "title looks like navigation or non-article text"
		return decision
	}
	if strings.TrimSpace(firstNonEmpty(item.CanonicalURL, item.RawURL)) == "" {
		decision.OK = false
		decision.Reason = "missing article URL"
		return decision
	}
	allowShort := source.SourceConfigValue(src, "allow_short_content", !src.FullContentAllowed)
	requireArticleContent := source.SourceConfigValue(src, "require_article_content", src.FullContentAllowed)
	minChars := source.SourceConfigValue(src, "min_content_chars", 0)
	if minChars <= 0 && src.FullContentAllowed {
		minChars = 300
	}
	minWords := source.SourceConfigValue(src, "min_word_count", 0)
	if minWords <= 0 && minChars > 0 {
		minWords = 45
	}
	if requireArticleContent && !allowShort && (decision.ContentChars < minChars || decision.WordCount < minWords) {
		decision.OK = false
		decision.Reason = "content below article-quality threshold"
		return decision
	}
	return decision
}

func looksLikeNavigationTitle(title string) bool {
	low := strings.ToLower(strings.TrimSpace(title))
	low = strings.Trim(low, " \t\n\r|-/—:.")
	if len(low) < 8 {
		return true
	}
	navTitles := map[string]bool{
		"home": true, "news": true, "newsroom": true, "press releases": true, "rss": true, "rss feeds": true,
		"subscribe": true, "search": true, "contact": true, "about": true, "careers": true, "events": true,
	}
	if navTitles[low] {
		return true
	}
	return strings.Contains(low, "subscribe to") || strings.Contains(low, "sign up") || strings.Contains(low, "email alerts") || strings.Contains(low, "skip to")
}

func normalizeItem(src db.Source, rawID uuid.UUID, item source.FetchedItem, canonical string, maxAge time.Duration) db.Article {
	id := rawID
	now := time.Now().UTC()
	if item.FetchedAt.IsZero() {
		item.FetchedAt = now
	}
	title := item.Title
	nt := normalize.NormalizeTitle(title)
	text := normalize.CleanText(firstNonEmpty(item.ContentText, item.Excerpt, item.Title))
	ch := normalize.ContentHash(text)
	var chp *string
	if ch != "" {
		chp = &ch
	}
	sim := int64(normalize.SimHash(text))
	pub := item.PublishedAt
	confidence := "high"
	outdated := false
	if pub == nil {
		confidence = "low"
		outdated = item.FetchedAt.Before(now.Add(-maxAge))
	} else {
		outdated = pub.Before(now.Add(-maxAge))
	}
	status := "normalized"
	if outdated {
		status = "outdated"
	}
	a := db.Article{SourceID: src.ID, RawItemID: &id, Title: title, NormalizedTitle: nt, Language: "en", PublishedAt: pub, FetchedAt: item.FetchedAt, TimeConfidence: confidence, Status: status, IsOutdated: outdated, TitleHash: normalize.SHA256Hex(nt), ContentHash: chp, Simhash: &sim, WordCount: normalize.WordCount(text), SourceCredibilityScore: src.CredibilityScore}
	if canonical != "" {
		a.CanonicalURL = &canonical
	}
	if item.Author != "" {
		a.Author = &item.Author
	}
	if item.Excerpt != "" {
		a.Excerpt = &item.Excerpt
	}
	if text != "" {
		a.ContentText = &text
	}
	if item.ContentHTML != "" && src.FullContentAllowed {
		a.ContentHTML = &item.ContentHTML
	}
	return a
}

func (s *Service) adapter(src db.Source) source.Adapter {
	switch src.Type {
	case "rss":
		return rss.New(src, s.Client, s.Config.Crawler.UserAgent)
	case "api":
		if src.Key == "federal_register_financial" {
			return federalreg.New(src, s.Client)
		}
		if src.Key == "sec_edgar_watchlist" {
			return sec.New(src, s.Client, s.Config.Crawler.UserAgent)
		}
		return federalreg.New(src, s.Client)
	case "html":
		return htmlsrc.New(src, s.Client, s.Config.Crawler.UserAgent)
	default:
		return nil
	}
}

type simpleErr string

func (e simpleErr) Error() string   { return string(e) }
func errUnsupported(t string) error { return simpleErr("unsupported source type: " + t) }
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func incMeta(metadata map[string]any, key string) {
	if metadata == nil {
		return
	}
	if v, ok := metadata[key]; ok {
		switch n := v.(type) {
		case int:
			metadata[key] = n + 1
		case int64:
			metadata[key] = n + 1
		case float64:
			metadata[key] = n + 1
		default:
			metadata[key] = 1
		}
		return
	}
	metadata[key] = 1
}

func metaCount(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch v := metadata[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func noArticleReason(metadata map[string]any, returned int) string {
	if returned == 0 {
		if v, ok := metadata["no_fresh_reason"].(string); ok && v != "" {
			return v
		}
		return "source_reachable_no_recent_items"
	}
	if metaCount(metadata, "quality_rejected_count") >= returned {
		return "all_candidates_failed_quality_gate"
	}
	if metaCount(metadata, "duplicate_count") >= returned {
		return "all_candidates_duplicate"
	}
	if metaCount(metadata, "outdated_after_normalization_count") >= returned {
		return "all_candidates_outdated"
	}
	return "no_new_articles_inserted"
}

type uuidLike interface{ UUID() uuid.UUID }
