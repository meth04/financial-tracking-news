package llm

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/cluster"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

type JobStore interface {
	PickLLMJob(context.Context, string) (*db.LLMJob, error)
	HasAnalysis(context.Context, uuid.UUID) (bool, error)
	GetArticle(context.Context, uuid.UUID) (db.Article, error)
	FailLLMJob(context.Context, uuid.UUID, string, time.Duration) error
	CompleteLLMJob(context.Context, uuid.UUID) error
	UpdateArticleStatus(context.Context, uuid.UUID, string) error
	SaveAnalysis(context.Context, db.Analysis) error
}

type Worker struct {
	Store          JobStore
	Analyzer       Analyzer
	Clusterer      cluster.Service
	MaxConcurrency int
	Backoffs       []time.Duration
	Logger         *slog.Logger
	WorkerID       string
	active         int32
}

func (w *Worker) Active() int { return int(atomic.LoadInt32(&w.active)) }

func (w *Worker) Start(ctx context.Context) error {
	if w.MaxConcurrency <= 0 {
		w.MaxConcurrency = 3
	}
	if w.WorkerID == "" {
		w.WorkerID = "finnews-worker"
	}
	if w.Logger == nil {
		w.Logger = slog.Default()
	}
	var wg sync.WaitGroup
	for i := 0; i < w.MaxConcurrency; i++ {
		wg.Add(1)
		go func(n int) { defer wg.Done(); w.loop(ctx, n) }(i)
	}
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (w *Worker) loop(ctx context.Context, n int) {
	for ctx.Err() == nil {
		processed, err := w.ProcessOne(ctx)
		if err != nil {
			w.Logger.Warn("llm job error", "worker", n, "error", err)
			time.Sleep(time.Second)
		}
		if !processed {
			time.Sleep(2 * time.Second)
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	job, err := w.Store.PickLLMJob(ctx, w.WorkerID)
	if err != nil || job == nil {
		return false, err
	}
	atomic.AddInt32(&w.active, 1)
	defer atomic.AddInt32(&w.active, -1)
	exists, err := w.Store.HasAnalysis(ctx, job.ArticleID)
	if err != nil {
		_ = w.Store.FailLLMJob(ctx, job.ID, err.Error(), w.backoff(job.Attempts))
		return true, err
	}
	if exists {
		_ = w.Store.CompleteLLMJob(ctx, job.ID)
		return true, nil
	}
	article, err := w.Store.GetArticle(ctx, job.ArticleID)
	if err != nil {
		_ = w.Store.FailLLMJob(ctx, job.ID, err.Error(), w.backoff(job.Attempts))
		return true, err
	}
	if article.IsOutdated {
		_ = w.Store.FailLLMJob(ctx, job.ID, "article outdated before LLM", 0)
		_ = w.Store.UpdateArticleStatus(ctx, article.ID, "outdated")
		return true, nil
	}
	ana, raw, err := w.Analyzer.AnalyzeArticle(ctx, article)
	if err != nil {
		_ = w.Store.FailLLMJob(ctx, job.ID, truncate(raw+": "+err.Error(), 2000), w.backoff(job.Attempts))
		return true, err
	}
	ana.ArticleID = article.ID
	if err := w.Store.SaveAnalysis(ctx, *ana); err != nil {
		_ = w.Store.FailLLMJob(ctx, job.ID, err.Error(), w.backoff(job.Attempts))
		return true, err
	}
	_ = w.Store.UpdateArticleStatus(ctx, article.ID, "llm_done")
	if w.Clusterer.Store != nil {
		_, _, _ = w.Clusterer.ClusterArticle(ctx, article, *ana)
	}
	return true, w.Store.CompleteLLMJob(ctx, job.ID)
}

func (w *Worker) backoff(attempts int) time.Duration {
	if len(w.Backoffs) == 0 {
		w.Backoffs = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	}
	if attempts < 0 {
		attempts = 0
	}
	if attempts >= len(w.Backoffs) {
		return w.Backoffs[len(w.Backoffs)-1]
	}
	return w.Backoffs[attempts]
}
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
