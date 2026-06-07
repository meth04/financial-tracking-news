package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	apihttp "github.com/nguyen/financial-tracking-news/internal/api"
	"github.com/nguyen/financial-tracking-news/internal/cluster"
	"github.com/nguyen/financial-tracking-news/internal/config"
	"github.com/nguyen/financial-tracking-news/internal/crawler"
	"github.com/nguyen/financial-tracking-news/internal/db"
	"github.com/nguyen/financial-tracking-news/internal/llm"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "migrate":
		if len(args) > 1 && args[1] == "up" {
			st, err := db.Connect(ctx, cfg)
			if err != nil {
				return err
			}
			defer st.Close()
			return st.Migrate(ctx, "db/schema.sql")
		}
	case "seed":
		if len(args) > 1 && args[1] == "sources" {
			st, err := db.Connect(ctx, cfg)
			if err != nil {
				return err
			}
			defer st.Close()
			sources, err := config.LoadSources("")
			if err != nil {
				return err
			}
			return st.SeedSources(ctx, sources)
		}
	case "crawl":
		if len(args) > 1 && args[1] == "once" {
			st, err := db.Connect(ctx, cfg)
			if err != nil {
				return err
			}
			defer st.Close()
			cr := crawler.New(st, cfg, logger)
			key := ""
			if len(args) > 2 {
				key = args[2]
			}
			return cr.CrawlOnce(ctx, key)
		}
	case "worker":
		st, err := db.Connect(ctx, cfg)
		if err != nil {
			return err
		}
		defer st.Close()
		analyzer := llm.NewOpenAIClient(cfg.LLM.BaseURL, cfg.LLMAPIKey(), cfg.LLM.Model, time.Duration(cfg.LLM.TimeoutSeconds)*time.Second)
		cl := cluster.Service{Store: st, FreshnessWindow: cfg.FreshnessDuration()}
		w := llm.Worker{Store: st, Analyzer: analyzer, Clusterer: cl, MaxConcurrency: cfg.LLM.MaxConcurrency, Backoffs: cfg.BackoffDurations(), Logger: logger, WorkerID: hostname()}
		return w.Start(ctx)
	case "cluster":
		if len(args) > 1 && args[1] == "repair" {
			logger.Info("cluster repair command is available; new analyses are clustered by worker in MVP")
			return nil
		}
	case "server":
		return runServer(ctx, cfg, logger)
	}
	usage()
	return nil
}

func runServer(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	st, err := db.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	cr := crawler.New(st, cfg, logger)
	cr.StartScheduler(ctx)
	analyzer := llm.NewOpenAIClient(cfg.LLM.BaseURL, cfg.LLMAPIKey(), cfg.LLM.Model, time.Duration(cfg.LLM.TimeoutSeconds)*time.Second)
	worker := llm.Worker{Store: st, Analyzer: analyzer, Clusterer: cluster.Service{Store: st, FreshnessWindow: cfg.FreshnessDuration()}, MaxConcurrency: cfg.LLM.MaxConcurrency, Backoffs: cfg.BackoffDurations(), Logger: logger, WorkerID: hostname()}
	go func() { _ = worker.Start(ctx) }()
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.App.Port), Handler: apihttp.New(st, cfg, cr, logger), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	logger.Info("starting server", "addr", srv.Addr, "llm_concurrency", cfg.LLM.MaxConcurrency)
	err = srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func usage() {
	fmt.Println(strings.TrimSpace(`Usage:
  finnews server
  finnews worker
  finnews crawl once [source_key]
  finnews migrate up
  finnews seed sources
  finnews cluster repair --fresh-only`))
}
func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		h = "finnews"
	}
	return h
}
