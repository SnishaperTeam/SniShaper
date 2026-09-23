//go:build !headless

package app

import (
	"context"
	"log"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Typed accessors for the GUI-only UI references (stored as `any` on App
// so the headless build never links the wails runtime).

func (a *App) wailsAppInstance() *application.App {
	if a.wailsApp == nil {
		return nil
	}
	return a.wailsApp.(*application.App)
}

func (a *App) mainWindowInstance() *application.WebviewWindow {
	if a.mainWindow == nil {
		return nil
	}
	return a.mainWindow.(*application.WebviewWindow)
}

func (a *App) systemTrayInstance() *application.SystemTray {
	if a.systemTray == nil {
		return nil
	}
	return a.systemTray.(*application.SystemTray)
}

func (a *App) trayMenuV3Instance() *application.Menu {
	if a.trayMenuV3 == nil {
		return nil
	}
	return a.trayMenuV3.(*application.Menu)
}

func (a *App) proxyItemV3Instance() *application.MenuItem {
	if a.proxyItemV3 == nil {
		return nil
	}
	return a.proxyItemV3.(*application.MenuItem)
}

func (a *App) systemProxyItemV3Instance() *application.MenuItem {
	if a.systemProxyItemV3 == nil {
		return nil
	}
	return a.systemProxyItemV3.(*application.MenuItem)
}

// invokeAsync runs fn on the wails event loop; the headless build
// executes it directly in its own goroutine.
func (a *App) invokeAsync(fn func()) {
	application.InvokeAsync(fn)
}

// SetWailsApp sets the wails application instance.
func (a *App) SetWailsApp(w *application.App) { a.wailsApp = w }

// SetMainWindow sets the main window reference.
func (a *App) SetMainWindow(w *application.WebviewWindow) {
	if w != nil {
		a.mainWindow = w
		if a.pendingShow {
			a.mainWindowInstance().Show()
			a.mainWindowInstance().Focus()
			a.pendingShow = false
		}
	} else {
		a.mainWindow = nil
		go func() {
			runtime.GC()
			debug.FreeOSMemory()
		}()
	}
}

// liveMainWindow returns the tracked main window while the runtime still knows
// about it, and drops the reference once it doesn't. The runtime removes a
// window from its window manager as part of destroying it, which happens on a
// goroutine of its own, so a close can land on this app after the native window
// is already gone. WebviewWindow methods are inert on such a window - Show()
// falls back to Run(), which returns immediately because impl is still set - so
// a stale reference would silently swallow every later attempt to bring the
// window back.
func (a *App) liveMainWindow() *application.WebviewWindow {
	if a.mainWindow == nil {
		return nil
	}
	w := a.mainWindowInstance()
	wailsApp := a.wailsAppInstance()
	if w == nil || wailsApp == nil {
		return w
	}
	if _, ok := wailsApp.Window.GetByID(w.ID()); !ok {
		log.Printf("[window] main window no longer registered, dropping stale reference")
		a.SetMainWindow(nil)
		return nil
	}
	return w
}

// hasMainWindow reports whether a live main window is tracked.
func (a *App) hasMainWindow() bool { return a.liveMainWindow() != nil }

// hasUI reports whether app state can be delivered to a UI. The desktop build
// needs a live window for that; the headless build (app_headless.go) pushes
// state to the adapter a frontend installs instead.
func (a *App) hasUI() bool { return a.hasMainWindow() }

func (a *App) IsHibernated() bool { return a.mainWindow == nil && !a.shouldQuit }

func (a *App) IsMainWindowVisible() bool {
	w := a.liveMainWindow()
	if w == nil {
		return false
	}
	return w.IsVisible()
}

func (a *App) HibernateMainWindow() {
	if a.mainWindow == nil {
		return
	}
	w := a.mainWindowInstance()
	log.Printf("[window] hibernate: releasing the frontend")
	// Release the reference first: the close handler treats a nil reference as
	// an app owned teardown and lets the runtime destroy the window.
	a.mainWindow = nil
	go func() {
		w.Close()
		time.Sleep(200 * time.Millisecond)
		runtime.GC()
		debug.FreeOSMemory()
		a.setTrayTooltip("SniShaper - 前端已休眠，点击托盘恢复")
	}()
}

func (a *App) ensureMainWindow() *application.WebviewWindow {
	if w := a.liveMainWindow(); w != nil {
		return w
	}
	wailsApp := a.wailsAppInstance()
	if wailsApp == nil {
		a.pendingShow = true
		return nil
	}
	log.Printf("[window] creating main window")
	w := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "snishaper",
		Width:            1024,
		Height:           768,
		URL:              "/",
		Frameless:        true,
		Hidden:           false,
		BackgroundColour: application.NewRGB(27, 38, 54),
	})
	a.AttachMainWindowHandlers(w)
	a.SetMainWindow(w)
	return w
}

// AttachMainWindowHandlers installs the close handling of a main window.
//
// The handler has to be a hook, not a listener. The runtime registers its own
// Common.WindowClosing listener that destroys the window unconditionally; hooks
// run synchronously and stop the dispatch before any listener goroutine is
// spawned, whereas listeners are all started concurrently. Cancelling the event
// from a listener therefore races the runtime's destroy handler and normally
// loses, which destroyed the window even with "close to tray" enabled and left
// the app without any way to show its UI again.
func (a *App) AttachMainWindowHandlers(w *application.WebviewWindow) {
	w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		a.handleWindowClosing(event, w)
	})
}

func (a *App) handleWindowClosing(event *application.WindowEvent, w *application.WebviewWindow) {
	if a.shouldQuit || a.mainWindow == nil {
		// Quitting, or a teardown this app already owns (hibernate): let the
		// runtime destroy the window.
		return
	}
	// Keep the close under app control so the runtime cannot destroy a window
	// the app still needs.
	event.Cancel()
	if a.GetHibernateOnClose() {
		a.HibernateMainWindow()
		return
	}
	if a.GetCloseToTray() {
		w.Hide()
		log.Printf("[window] close: hidden to tray")
		return
	}
	log.Printf("[window] close: quitting application")
	a.QuitApp()
}

// SetSystemTray sets the system tray reference.
func (a *App) SetSystemTray(t *application.SystemTray) { a.systemTray = t }

// SetTrayMenu sets the tray menu reference.
func (a *App) SetTrayMenu(m *application.Menu) { a.trayMenuV3 = m }

// SetProxyMenuItem sets the proxy menu item reference.
func (a *App) SetProxyMenuItem(i *application.MenuItem) { a.proxyItemV3 = i }

// SetSystemProxyMenuItem sets the system proxy menu item reference.
func (a *App) SetSystemProxyMenuItem(i *application.MenuItem) { a.systemProxyItemV3 = i }

// wails service lifecycle entry points.

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.startupV3()
	return nil
}

func (a *App) ServiceShutdown() error {
	stopInstanceListener()
	a.shutdown()
	return nil
}

// Shared UI operations: the GUI build dispatches onto wails; the headless
// build (app_headless.go) provides no-op equivalents, forwarding events to
// the UIAdapter when one is installed.

func (a *App) emitEvent(event string, payload interface{}) {
	if w := a.liveMainWindow(); w != nil {
		w.EmitEvent(event, payload)
	}
}

func (a *App) showMainWindow() {
	if w := a.liveMainWindow(); w != nil {
		w.Show()
		w.Focus()
		return
	}
	if w := a.ensureMainWindow(); w != nil {
		w.Show()
		w.Focus()
	}
}

func (a *App) hideMainWindow() {
	if w := a.liveMainWindow(); w != nil {
		w.Hide()
	}
}

func (a *App) minimiseMainWindow() {
	if w := a.liveMainWindow(); w != nil {
		w.Minimise()
	}
}

func (a *App) toggleMaximiseMainWindow() {
	if w := a.liveMainWindow(); w != nil {
		w.ToggleMaximise()
	}
}

func (a *App) closeMainWindow() {
	if w := a.liveMainWindow(); w != nil {
		w.Close()
	}
}

func (a *App) quitAppUI() {
	if a.wailsApp != nil {
		a.wailsAppInstance().Quit()
	}
}

func (a *App) setTrayTooltip(text string) {
	if a.systemTray != nil {
		a.systemTrayInstance().SetTooltip(text)
	}
}

func (a *App) updateTrayProxyItem(label string, checked bool) {
	if a.proxyItemV3 != nil {
		a.proxyItemV3Instance().SetLabel(label)
		a.proxyItemV3Instance().SetChecked(checked)
	}
}

func (a *App) updateTraySysProxyItem(label string) {
	if a.systemProxyItemV3 != nil {
		a.systemProxyItemV3Instance().SetLabel(label)
	}
}
