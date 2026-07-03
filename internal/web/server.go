package web

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/valpere/depl-orch/internal/agent"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Server struct {
	addr    string
	log     *slog.Logger
	workDir string
	tmpl    *template.Template
	server  *http.Server
}

func NewServer(addr, workDir string, log *slog.Logger) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		addr:    addr,
		log:     log,
		workDir: workDir,
		tmpl:    tmpl,
	}, nil
}

func (s *Server) Name() string {
	return "dashboard"
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleDashboard)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	s.log.Info("dashboard server listening", "addr", s.addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	historyPath := filepath.Join(s.workDir, ".agents", "fix-history.jsonl")
	var history []agent.HistoryItem

	if f, err := os.Open(historyPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var item agent.HistoryItem
			if err := json.Unmarshal(scanner.Bytes(), &item); err == nil {
				history = append([]agent.HistoryItem{item}, history...) // prepend so latest is first
			}
		}
	}

	data := struct {
		History []agent.HistoryItem
	}{
		History: history,
	}

	if err := s.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		s.log.Error("template execute", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
