package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"mango3s/internal/api"
	"mango3s/internal/config"
	"mango3s/internal/logging"
	"mango3s/internal/mongox"
	"mango3s/internal/secrets"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	run := runDesktop
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		run = runMCP // `mango3s mcp`: stdio MCP server for Claude Desktop / Claude Code
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Mango3S:", err)
		os.Exit(1)
	}
}

// newApp wires the shared dependencies for both the desktop app and the MCP server.
func newApp() (*api.App, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, fmt.Errorf("config directory: %w", err)
	}
	log, level, logPath, err := logging.Setup(dir, slog.LevelInfo)
	if err != nil {
		log.Warn("file logging unavailable", "err", err)
	}
	sec, err := secrets.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("secret store: %w", err)
	}
	log.Info("secret store selected", "backend", sec.Backend())
	return api.New(api.Deps{
		Log:      log,
		LogLevel: level,
		LogPath:  logPath,
		Store:    config.NewStore(dir),
		Secrets:  sec,
		Conns:    mongox.NewManager(log),
	}), nil
}

// runMCP serves the approved MCP tools on stdin/stdout until the client disconnects.
func runMCP() error {
	app, err := newApp()
	if err != nil {
		return err
	}
	srv, err := api.NewMCPServer(app)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer api.NewLifecycle(app).Shutdown(context.Background())
	return srv.Run(ctx)
}

func runDesktop() error {
	app, err := newApp()
	if err != nil {
		return err
	}
	lc := api.NewLifecycle(app)

	return wails.Run(&options.App{
		Title:     "Mango3S",
		Width:     1360,
		Height:    860,
		MinWidth:  900,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 26, B: 31, A: 1},
		OnStartup:        lc.Startup,
		OnShutdown:       lc.Shutdown,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "Mango3S",
				Message: "A MongoDB IDE · v" + api.Version,
			},
		},
	})
}
