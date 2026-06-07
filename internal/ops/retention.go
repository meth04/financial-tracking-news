package ops

import (
	"context"
	"fmt"

	"github.com/nguyen/financial-tracking-news/internal/config"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

func RunRetention(ctx context.Context, store *db.Store, cfg config.Config) error {
	if cfg.Storage.RawRetentionDays > 0 {
		if _, err := store.DB.ExecContext(ctx, fmt.Sprintf("DELETE FROM raw_items WHERE created_at < datetime('now','-%d days')", cfg.Storage.RawRetentionDays)); err != nil {
			return err
		}
	}
	if cfg.Storage.ArticleRetentionDays > 0 {
		if _, err := store.DB.ExecContext(ctx, fmt.Sprintf("DELETE FROM articles WHERE created_at < datetime('now','-%d days')", cfg.Storage.ArticleRetentionDays)); err != nil {
			return err
		}
	}
	if cfg.Storage.ClusterRetentionDays > 0 {
		if _, err := store.DB.ExecContext(ctx, fmt.Sprintf("DELETE FROM event_clusters WHERE created_at < datetime('now','-%d days')", cfg.Storage.ClusterRetentionDays)); err != nil {
			return err
		}
	}
	return nil
}
