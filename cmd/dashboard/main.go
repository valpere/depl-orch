package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/valpere/depl-orch/internal/framework/lifecycle"
	"github.com/valpere/depl-orch/internal/web"
)

func main() {
	addr := flag.String("addr", ":8082", "Address to listen on")
	workDir := flag.String("workdir", ".", "Working directory of the repository")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app := lifecycle.New("dashboard", log)

	srv, err := web.NewServer(*addr, *workDir, log)
	if err != nil {
		log.Error("failed to create server", "err", err)
		os.Exit(1)
	}

	app.Register(srv)

	if err := app.Run(ctx); err != nil {
		log.Error("dashboard failed", "err", err)
		os.Exit(1)
	}
}
