package llm

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

type fakeAnalyzer struct {
	active int32
	max    int32
	delay  time.Duration
}

func (f *fakeAnalyzer) AnalyzeArticle(ctx context.Context, a db.Article) (*db.Analysis, string, error) {
	n := atomic.AddInt32(&f.active, 1)
	for {
		m := atomic.LoadInt32(&f.max)
		if n <= m || atomic.CompareAndSwapInt32(&f.max, m, n) {
			break
		}
	}
	time.Sleep(f.delay)
	atomic.AddInt32(&f.active, -1)
	return &db.Analysis{ArticleID: a.ID, EventTitle: "Event", EventType: "other", MarketImpact: "low", Sentiment: "neutral", KeyFacts: []byte("[]"), NewInformation: []byte("[]"), RawJSON: []byte("{}")}, "{}", nil
}

type fakeJobStore struct {
	mu   sync.Mutex
	jobs []db.LLMJob
	done int
}

func (s *fakeJobStore) PickLLMJob(ctx context.Context, worker string) (*db.LLMJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.jobs) == 0 {
		return nil, nil
	}
	j := s.jobs[0]
	s.jobs = s.jobs[1:]
	return &j, nil
}
func (s *fakeJobStore) HasAnalysis(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (s *fakeJobStore) GetArticle(ctx context.Context, id uuid.UUID) (db.Article, error) {
	return db.Article{ID: id, Title: "A", FetchedAt: time.Now()}, nil
}
func (s *fakeJobStore) FailLLMJob(context.Context, uuid.UUID, string, time.Duration) error {
	return nil
}
func (s *fakeJobStore) CompleteLLMJob(context.Context, uuid.UUID) error {
	s.mu.Lock()
	s.done++
	s.mu.Unlock()
	return nil
}
func (s *fakeJobStore) UpdateArticleStatus(context.Context, uuid.UUID, string) error { return nil }
func (s *fakeJobStore) SaveAnalysis(context.Context, db.Analysis) error              { return nil }

func TestWorkerMaxConcurrency(t *testing.T) {
	store := &fakeJobStore{}
	for i := 0; i < 8; i++ {
		store.jobs = append(store.jobs, db.LLMJob{ID: uuid.New(), ArticleID: uuid.New(), Attempts: 0, MaxAttempts: 5})
	}
	analyzer := &fakeAnalyzer{delay: 30 * time.Millisecond}
	w := Worker{Store: store, Analyzer: analyzer, MaxConcurrency: 3, WorkerID: "test"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	_ = w.Start(ctx)
	if analyzer.max > 3 {
		t.Fatalf("max concurrency %d", analyzer.max)
	}
	if store.done == 0 {
		t.Fatal("expected jobs processed")
	}
}
