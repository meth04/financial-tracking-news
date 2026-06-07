package dedup

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

type fakeStore struct {
	dup *db.Article
	typ string
}

func (f fakeStore) FindExactDuplicate(context.Context, db.Article, time.Duration) (*db.Article, string, error) {
	return f.dup, f.typ, nil
}

func TestExactDuplicateByURL(t *testing.T) {
	id := uuid.New()
	d := db.Article{ID: id}
	e := Engine{Store: fakeStore{dup: &d, typ: "url"}, Window: 72 * time.Hour}
	got, err := e.Decide(context.Background(), db.Article{})
	if err != nil || got.Kind != KindExactDuplicate || got.DuplicateOf == nil || *got.DuplicateOf != id {
		t.Fatalf("bad decision %#v err %v", got, err)
	}
}
func TestSameSourceT1UpdateCandidate(t *testing.T) {
	src := uuid.New()
	oldHash := "old"
	newHash := "new"
	oldPub := time.Now().Add(-time.Hour)
	newPub := time.Now()
	d := db.Article{ID: uuid.New(), SourceID: src, ContentHash: &oldHash, PublishedAt: &oldPub, WordCount: 100}
	e := Engine{Store: fakeStore{dup: &d, typ: "title"}, Window: 72 * time.Hour}
	got, _ := e.Decide(context.Background(), db.Article{SourceID: src, ContentHash: &newHash, PublishedAt: &newPub, WordCount: 120})
	if got.Kind != KindCandidateUpdate {
		t.Fatalf("got %s", got.Kind)
	}
}
func TestUpdateClassification(t *testing.T) {
	if ClassifyUpdate(false, true, 50, true, .4) != "update" {
		t.Fatal("expected update")
	}
	if ClassifyUpdate(false, true, 5, false, .95) != "duplicate" {
		t.Fatal("expected duplicate")
	}
	if ClassifyUpdate(false, true, 25, false, .4) != "related" {
		t.Fatal("expected related")
	}
}
func TestNearDuplicateDecision(t *testing.T) {
	got := NearDuplicateByText("fed holds rates", "fed holds rates steady", "fed holds rates and signals cuts", "fed holds rates and signals cuts", 1, 1)
	if got.Kind != KindNearDuplicate {
		t.Fatalf("expected near duplicate got %#v", got)
	}
}
