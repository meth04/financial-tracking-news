package dedup

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/normalize"
)

const (
	KindNew             = "new"
	KindExactDuplicate  = "exact_duplicate"
	KindNearDuplicate   = "near_duplicate"
	KindCandidateUpdate = "candidate_update"
)

type Decision struct {
	Kind        string
	DuplicateOf *uuid.UUID
	Similarity  float64
	Reason      string
}

type Store interface {
	FindExactDuplicate(context.Context, db.Article, time.Duration) (*db.Article, string, error)
}

type Engine struct {
	Store  Store
	Window time.Duration
}

func (e Engine) Decide(ctx context.Context, a db.Article) (Decision, error) {
	if e.Window == 0 {
		e.Window = 72 * time.Hour
	}
	dup, typ, err := e.Store.FindExactDuplicate(ctx, a, e.Window)
	if err != nil {
		return Decision{}, err
	}
	if dup == nil {
		return Decision{Kind: KindNew, Reason: "no exact duplicate found"}, nil
	}
	// Same-source same-title but later and different content remains eligible for LLM/update detection.
	if typ == "title" && dup.SourceID == a.SourceID && a.ContentHash != nil && dup.ContentHash != nil && *a.ContentHash != *dup.ContentHash {
		if a.PublishedAt != nil && (dup.PublishedAt == nil || a.PublishedAt.After(*dup.PublishedAt)) && a.WordCount >= dup.WordCount {
			return Decision{Kind: KindCandidateUpdate, DuplicateOf: &dup.ID, Similarity: 0.65, Reason: "same source/title but newer content may contain T+1 update"}, nil
		}
	}
	return Decision{Kind: KindExactDuplicate, DuplicateOf: &dup.ID, Similarity: 1, Reason: "exact duplicate by " + typ}, nil
}

func NearDuplicateByText(aTitle, aContent, bTitle, bContent string, aSim, bSim uint64) Decision {
	titleSim := normalize.Similarity(aTitle, bTitle)
	contentSim := normalize.JaccardShingles(aContent, bContent, 5)
	ham := normalize.Hamming(aSim, bSim)
	if titleSim >= 0.92 || ham <= 3 || contentSim >= 0.90 {
		return Decision{Kind: KindNearDuplicate, Similarity: max(titleSim, contentSim), Reason: "high title/content/simhash similarity"}
	}
	if ham <= 8 || titleSim >= 0.65 {
		return Decision{Kind: KindCandidateUpdate, Similarity: max(titleSim, contentSim), Reason: "same-event candidate by similarity"}
	}
	return Decision{Kind: KindNew, Similarity: max(titleSim, contentSim), Reason: "similarity below threshold"}
}

func ClassifyUpdate(exact bool, sameEvent bool, novelty int, hasNewFacts bool, contentSimilarity float64) string {
	if exact {
		return "duplicate"
	}
	if sameEvent {
		if novelty >= 35 && hasNewFacts {
			return "update"
		}
		if contentSimilarity >= 0.90 && !hasNewFacts {
			return "duplicate"
		}
		return "related"
	}
	return "original"
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
