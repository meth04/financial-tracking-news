package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/nguyen/financial-tracking-news/internal/config"
	"github.com/nguyen/financial-tracking-news/internal/crawler"
	"github.com/nguyen/financial-tracking-news/internal/db"
)

type Server struct {
	Store   *db.Store
	Crawler *crawler.Service
	Config  config.Config
	Log     *slog.Logger
}

func New(s *db.Store, cfg config.Config, cr *crawler.Service, log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	srv := &Server{Store: s, Config: cfg, Crawler: cr, Log: log}
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{AllowedOrigins: cfg.API.CORSAllowedOrigins, AllowedMethods: []string{"GET", "POST", "OPTIONS"}, AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"}}))
	r.Get("/api/health", srv.health)
	r.Get("/api/stats", srv.stats)
	r.Get("/api/articles", srv.articles)
	r.Get("/api/articles/{id}", srv.article)
	r.Get("/api/clusters", srv.clusters)
	r.Get("/api/clusters/{id}", srv.cluster)
	r.Get("/api/sources", srv.sources)
	r.Get("/api/jobs/llm", srv.llmJobs)
	r.Post("/api/admin/crawl-once", srv.crawlOnce)
	r.Post("/api/admin/retry-failed-llm", srv.retryLLM)
	r.Get("/*", srv.static)
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, 200, map[string]any{"status": "ok", "database": "not_configured"})
		return
	}
	writeJSON(w, 200, s.Store.Health(r.Context()))
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	v, err := s.Store.Stats(r.Context())
	if err != nil {
		writeErr(w, 500, "stats unavailable")
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) sources(w http.ResponseWriter, r *http.Request) {
	v, err := s.Store.ListSourceHealth(r.Context())
	if err != nil {
		writeErr(w, 500, "sources unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"items": v})
}
func (s *Server) llmJobs(w http.ResponseWriter, r *http.Request) {
	v, err := s.Store.LLMQueueStatus(r.Context())
	if err != nil {
		writeErr(w, 500, "queue unavailable")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) articles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := db.ArticleFilters{Q: q.Get("q"), Source: q.Get("source"), Ticker: q.Get("ticker"), EventType: q.Get("event_type"), Impact: q.Get("impact"), Sentiment: q.Get("sentiment"), Status: q.Get("status"), FreshOnly: parseBoolDefault(q.Get("fresh_only"), true), Page: parseIntDefault(q.Get("page"), 1), PageSize: clamp(parseIntDefault(q.Get("page_size"), s.Config.API.PageSizeDefault), 1, s.Config.API.PageSizeMax), Sort: q.Get("sort"), Order: q.Get("order")}
	res, err := s.Store.ListArticles(r.Context(), f)
	if err != nil {
		s.Log.Error("list articles", "error", err)
		writeErr(w, 500, "articles unavailable")
		return
	}
	writeJSON(w, 200, res)
}
func (s *Server) article(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	a, err := s.Store.GetArticle(r.Context(), id)
	if err != nil {
		writeErr(w, 404, "article not found")
		return
	}
	writeJSON(w, 200, a)
}
func (s *Server) clusters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := db.ClusterFilters{Q: q.Get("q"), Ticker: q.Get("ticker"), EventType: q.Get("event_type"), ImpactMin: parseIntDefault(q.Get("impact_min"), 0), FreshOnly: parseBoolDefault(q.Get("fresh_only"), true), Page: parseIntDefault(q.Get("page"), 1), PageSize: clamp(parseIntDefault(q.Get("page_size"), s.Config.API.PageSizeDefault), 1, s.Config.API.PageSizeMax), Sort: q.Get("sort"), Order: q.Get("order")}
	res, err := s.Store.ListClusters(r.Context(), f)
	if err != nil {
		writeErr(w, 500, "clusters unavailable")
		return
	}
	writeJSON(w, 200, res)
}
func (s *Server) cluster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	c, err := s.Store.GetCluster(r.Context(), id)
	if err != nil {
		writeErr(w, 404, "cluster not found")
		return
	}
	writeJSON(w, 200, c)
}

func (s *Server) crawlOnce(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceKey string `json:"source_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if s.Crawler == nil {
		writeErr(w, 503, "crawler disabled")
		return
	}
	go func() { _ = s.Crawler.CrawlOnce(context.Background(), strings.TrimSpace(req.SourceKey)) }()
	writeJSON(w, 202, map[string]any{"status": "accepted"})
}
func (s *Server) retryLLM(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.RetryFailedLLM(r.Context())
	if err != nil {
		writeErr(w, 500, "retry failed")
		return
	}
	writeJSON(w, 202, map[string]any{"reset": n})
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	distDir, ok := staticDistDir()
	if !ok {
		http.NotFound(w, r)
		return
	}
	indexPath := filepath.Join(distDir, "index.html")
	cleanPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if cleanPath == "/" {
		http.ServeFile(w, r, indexPath)
		return
	}
	rel := strings.TrimPrefix(cleanPath, "/")
	target := filepath.Join(distDir, filepath.FromSlash(rel))
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return
	}
	if path.Ext(cleanPath) != "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, indexPath)
}

func staticDistDir() (string, bool) {
	for _, dir := range []string{
		filepath.Join("web", "dist"),
		filepath.Join("..", "web", "dist"),
		filepath.Join("..", "..", "web", "dist"),
	} {
		indexPath := filepath.Join(dir, "index.html")
		if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
			return dir, true
		}
	}
	return "", false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}
func parseBoolDefault(s string, def bool) bool {
	if s == "" {
		return def
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return v
}
func parseIntDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
