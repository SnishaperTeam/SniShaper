package main

import (
	"embed"
	"log"

	"snishaper/app"
	"snishaper/core"

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

	app.RecoverBrokenSingleInstance("com.snishaper.desktop")

	a := app.NewApp()

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

	// Create Tray
	tray := wailsApp.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetDarkModeIcon(trayIcon)
	tray.SetTooltip("SniShaper")
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
