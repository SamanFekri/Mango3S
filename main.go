package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
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

// appMenu is the native menu. Items forward to the frontend's shortcut
// commands, so the menu and the keyboard run the same code. The Edit menu
// provides the standard clipboard and undo actions inside the web view
// (required on macOS for Cmd+C/V/X/Z to work).
func appMenu(app *api.App) *menu.Menu {
	m := menu.NewMenu()
	emit := func(name string) func(*menu.CallbackData) {
		return func(*menu.CallbackData) { api.MenuEvent(app, name) }
	}
	mac := goruntime.GOOS == "darwin"
	if mac {
		// Our own application menu (macOS uses the first menu as the app
		// menu): Wails' built-in one has no place for Settings… ⌘,.
		am := m.AddSubmenu("Mango3S")
		am.AddText("About Mango3S", nil, emit("about"))
		am.AddSeparator()
		am.AddText("Settings…", keys.CmdOrCtrl(","), emit("settings"))
		am.AddText("Keymap…", nil, emit("keymap"))
		am.AddSeparator()
		am.AddText("Hide Mango3S", keys.CmdOrCtrl("h"), func(*menu.CallbackData) { api.HideApp(app) })
		am.AddSeparator()
		am.AddText("Quit Mango3S", keys.CmdOrCtrl("q"), func(*menu.CallbackData) { api.QuitApp(app) })
	}
	file := m.AddSubmenu("File")
	file.AddText("Connection Manager…", nil, emit("connection-manager"))
	file.AddText("New Connection…", nil, emit("new-connection"))
	file.AddSeparator()
	file.AddText("Close Tab", nil, emit("close-tab"))
	if !mac {
		file.AddSeparator()
		file.AddText("Settings…", nil, emit("settings"))
		file.AddSeparator()
		file.AddText("Quit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) { api.QuitApp(app) })
	}
	m.Append(menu.EditMenu())
	view := m.AddSubmenu("View")
	view.AddText("Welcome", nil, emit("welcome"))
	view.AddText("Library", nil, emit("library"))
	view.AddText("Scheduled Tasks", nil, emit("tasks"))
	if mac {
		m.Append(menu.WindowMenu())
	}
	help := m.AddSubmenu("Help")
	help.AddText("Keyboard Shortcuts", nil, emit("shortcuts"))
	help.AddText("Welcome", nil, emit("welcome"))
	if !mac {
		help.AddSeparator()
		help.AddText("About Mango3S", nil, emit("about"))
	}
	return m
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
		OnBeforeClose:    api.BeforeClose(app),
		Menu:             appMenu(app),
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "Mango3S",
				Message: "A desktop IDE for MongoDB · v" + api.Version,
			},
		},
	})
}
