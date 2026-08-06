package main

import (
	"embed"
	"log"

	"snishaper/app"
	"snishaper/core"
	"snishaper/pkg/sysproxy"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

func main() {
	if app.HasLaunchArg("--core") {
		if err := core.RunCoreMain(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if app.HasLaunchArg("--elevated") {
		// Already elevated, continue
	} else if !core.IsProcessElevated() {
		// Already running? Don't trigger a UAC prompt for a second instance.
		if app.IsSingleInstanceRunning("com.snishaper.desktop") {
			log.Printf("[main] Instance already running, skipping auto-elevate")
		} else if err := core.ElevateSelf(); err != nil {
			log.Printf("[main] Auto-elevate failed: %v, continuing without admin", err)
		} else {
			return // elevated instance will start
		}
	}

	app.RecoverBrokenSingleInstance("com.snishaper.desktop")

	// Wake the running instance; if the wake fails (e.g. stale instance without
	// the cross-integrity message filter), kill it and take over.
	if app.IsSingleInstanceRunning("com.snishaper.desktop") {
		if err := app.WakeSingleInstance("com.snishaper.desktop"); err == nil {
			log.Printf("[main] Woke running instance, exiting second instance")
			return
		}
		log.Printf("[main] Failed to wake running instance, killing stale instance and taking over")
		app.KillSingleInstance("com.snishaper.desktop")
	}

	a := app.NewApp()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[main] Panic recovered, forcing cleanup: %v", r)
		}
		a.ForceCleanup()
		// Force-disable system proxy even if ForceCleanup missed it
		_ = sysproxy.DisableSystemProxy()
	}()

	wailsApp := application.New(application.Options{
		Name:        "snishaper",
		Description: "SniShaper - Cloudflare IP Shaper",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Services: []application.Service{
			application.NewService(a),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.snishaper.desktop",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				a.RevealMainWindow()
			},
			ExitCode: 0,
		},
		Icon: trayIcon,
	})

	a.SetWailsApp(wailsApp)

	// Allow WM_COPYDATA from lower-integrity (non-elevated) second instances
	app.AllowSingleInstanceCrossIntegrity("com.snishaper.desktop")

	// Create Tray
	tray := wailsApp.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetDarkModeIcon(trayIcon)
	tray.SetTooltip("SniShaper")
	// ponytail: single click on tray icon shows main window
	tray.OnClick(func() {
		a.RevealMainWindow()
	})
	a.SetSystemTray(tray)

	// Define Tray Menu
	trayMenu := application.NewMenu()
	trayMenu.Add("仪表盘").OnClick(func(ctx *application.Context) {
		a.RevealMainWindow()
	})
	trayMenu.AddSeparator()

	proxyLabel := "代理: 关"
	if a.IsProxyRunning() {
		proxyLabel = "代理: 开"
	}
	proxyItem := trayMenu.AddCheckbox(proxyLabel, a.IsProxyRunning())
	proxyItem.OnClick(func(ctx *application.Context) {
		a.RunSafeAsync("tray proxy toggle", func() {
			if a.IsProxyRunning() {
				_ = a.StopProxy()
			} else {
				_ = a.StartProxy()
			}
		})
	})
	a.SetProxyMenuItem(proxyItem)

	systemProxyLabel := "系统代理: 关"
	if a.GetSystemProxyStatus().Enabled {
		systemProxyLabel = "系统代理: 开"
	}
	systemProxyItem := trayMenu.Add(systemProxyLabel)
	systemProxyItem.OnClick(func(ctx *application.Context) {
		a.RunSafeAsync("tray system proxy toggle", func() {
			if a.GetSystemProxyStatus().Enabled {
				_ = a.DisableSystemProxy()
				return
			}
			if !a.IsProxyRunning() {
				if err := a.StartProxy(); err != nil {
					return
				}
			}
			_ = a.EnableSystemProxy()
		})
	})
	a.SetSystemProxyMenuItem(systemProxyItem)

	trayMenu.AddSeparator()
	trayMenu.Add("退出").OnClick(func(ctx *application.Context) {
		a.QuitApp()
	})

	tray.SetMenu(trayMenu)
	a.SetTrayMenu(trayMenu)

	// Create Main Window
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "snishaper",
		Width:            1024,
		Height:           768,
		URL:              "/",
		Frameless:        true,
		Hidden:           a.ShouldStartHidden(),
		BackgroundColour: application.NewRGB(27, 38, 54),
	})
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !a.ShouldQuit() {
			if a.GetCloseToTray() {
				event.Cancel()
				mainWindow.Hide()
			} else {
				a.QuitApp()
			}
		}
	})
	a.SetMainWindow(mainWindow)

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
