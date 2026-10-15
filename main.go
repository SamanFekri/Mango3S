package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"

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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Mango3S:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := config.Dir()
	if err != nil {
		return fmt.Errorf("config directory: %w", err)
	}
	log, level, logPath, err := logging.Setup(dir, slog.LevelInfo)
	if err != nil {
		log.Warn("file logging unavailable", "err", err)
	}
	sec, err := secrets.Open(dir)
	if err != nil {
		return fmt.Errorf("secret store: %w", err)
	}
	log.Info("secret store selected", "backend", sec.Backend())

	app := api.New(api.Deps{
		Log:      log,
		LogLevel: level,
		LogPath:  logPath,
		Store:    config.NewStore(dir),
		Secrets:  sec,
		Conns:    mongox.NewManager(log),
	})
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
