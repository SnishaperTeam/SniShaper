package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

func main() {
	if hasLaunchArg("--core") {
		if err := runCoreMain(); err != nil {
			log.Fatal(err)
		}
		return
	}

	recoverBrokenSingleInstance("com.snishaper.desktop")

	app := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "snishaper",
		Description: "SniShaper - Cloudflare IP Shaper",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Services: []application.Service{
			application.NewService(app),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.snishaper.desktop",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				app.RevealMainWindow()
			},
			ExitCode: 0,
		},
	})

	app.wailsApp = wailsApp

	// Create Tray
	tray := wailsApp.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetDarkModeIcon(trayIcon)
	tray.SetTooltip("SniShaper")
	app.systemTray = tray

	// Define Tray Menu
	trayMenu := application.NewMenu()
	trayMenu.Add("仪表盘").OnClick(func(ctx *application.Context) {
		app.RevealMainWindow()
	})
	trayMenu.AddSeparator()

	proxyLabel := "代理: 关"
	if app.IsProxyRunning() {
		proxyLabel = "代理: 开"
	}
	app.proxyItemV3 = trayMenu.AddCheckbox(proxyLabel, app.IsProxyRunning())
	app.proxyItemV3.OnClick(func(ctx *application.Context) {
		app.runSafeAsync("tray proxy toggle", func() {
			if app.IsProxyRunning() {
				_ = app.StopProxy()
			} else {
				_ = app.StartProxy()
			}
		})
	})

	systemProxyLabel := "系统代理: 关"
	if app.GetSystemProxyStatus().Enabled {
		systemProxyLabel = "系统代理: 开"
	}
	app.systemProxyItemV3 = trayMenu.Add(systemProxyLabel)
	app.systemProxyItemV3.OnClick(func(ctx *application.Context) {
		app.runSafeAsync("tray system proxy toggle", func() {
			if app.GetSystemProxyStatus().Enabled {
				_ = app.DisableSystemProxy()
				return
			}
			if !app.IsProxyRunning() {
				if err := app.StartProxy(); err != nil {
					return
				}
			}
			_ = app.EnableSystemProxy()
		})
	})

	warpLabel := "Warp: 关"
	if app.GetWarpStatus().Running {
		warpLabel = "Warp: 开"
	}
	app.warpItemV3 = trayMenu.AddCheckbox(warpLabel, app.GetWarpStatus().Running)
	app.warpItemV3.OnClick(func(ctx *application.Context) {
		app.runSafeAsync("tray warp toggle", func() {
			status := app.GetWarpStatus()
			if status.Running {
				_ = app.StopWarp()
			} else {
				_ = app.StartWarp()
			}
		})
	})

	trayMenu.AddSeparator()
	trayMenu.Add("退出").OnClick(func(ctx *application.Context) {
		app.QuitApp()
	})

	tray.SetMenu(trayMenu)
	app.trayMenuV3 = trayMenu

	// Create Main Window
	app.mainWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "snishaper",
		Width:            1024,
		Height:           768,
		URL:              "/",
		Frameless:        true,
		Hidden:           app.ShouldStartHidden(),
		BackgroundColour: application.NewRGB(27, 38, 54),
	})
	tray.OnClick(func() {
		app.RevealMainWindow()
	})

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
