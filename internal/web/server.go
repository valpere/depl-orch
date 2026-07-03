package web

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net"
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

	// Bind synchronously so the port is reserved (and Stop's Shutdown call is
	// meaningful) before Start returns — Serve on an already-open listener
	// can't race with a caller that observes ctx.Done() and calls Stop.
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.log.Info("dashboard server listening", "addr", s.addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
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
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // diffs can exceed the 64KB default
		for scanner.Scan() {
			var item agent.HistoryItem
			if err := json.Unmarshal(scanner.Bytes(), &item); err == nil {
				history = append(history, item)
			}
		}
		// Reverse in place so the most recent item is first (O(n), was O(n²) via repeated prepend).
		for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
			history[i], history[j] = history[j], history[i]
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
