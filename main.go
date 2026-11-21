package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hy-shine/dipo/internal/colour"
	"github.com/hy-shine/dipo/internal/library"
	"github.com/hy-shine/dipo/locales"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/updater"
)

//go:embed resources/tray-template.png
var trayTemplate []byte

type localizedText func(locales.Messages) string

func (text localizedText) in(messages locales.Messages) string {
	if text == nil {
		return ""
	}
	return text(messages)
}

type dipo struct {
	scope              *core.Scope
	revision           *core.Signal[int]
	window             *desktop.Window
	settingsWindow     *desktop.Window
	showSettingsWindow func()
	toast              *desktop.Window
	tray               *desktop.Tray
	workers            sync.WaitGroup
	pick               func()
	state              library.State
	store              *library.Store
	current            colour.Color
	original           colour.Color
	input              string
	inputErr           localizedText
	storageErr         localizedText
	page               int
	model              int
	openAtLogin        bool
	launchAtLogin      func(done func(on bool))
	setLaunchAtLogin   func(on bool)
	recentClearPrompt  bool
	recording          bool
	shortcutErr        localizedText
	shortcutField      *core.Ref
	settingsScroll     core.Ref
	background         colour.Color
	backgroundInput    string
	backgroundErr      localizedText
	hue                float64
	saturation         float64
	brightness         float64
	writeClipboard     func(string) error
	restoreMain        bool
	permissionChecked  bool
	permissionGranted  bool
	permissionChecking bool
	permissionError    localizedText
	permissionPrompt   bool
	sampling           bool
	quitting           bool
	status             localizedText
	statusFailed       bool
	copyNotice         bool
	hotkey             *desktop.HotKey
	hotkeyScope        *core.Scope
	registeredShortcut string
	shortcutPending    bool
	stopCopyFeedback   func() bool
	stopToast          func() bool
}

func newDipo(scope *core.Scope) *dipo {
	app := &dipo{
		scope: scope, state: library.New(),
		background: colour.Opaque(255, 255, 255), backgroundInput: "#FFFFFF",
	}
	if scope != nil {
		app.revision = core.NewSignal(scope, 0)
	}
	app.setColor(colour.Opaque(19, 22, 27), true)
	return app
}

func (app *dipo) observe() {
	if app.revision != nil {
		app.revision.Get()
	}
}

func (app *dipo) notify() {
	if app.revision != nil {
		app.revision.Update(func(value int) int { return value + 1 })
	}
}

func (app *dipo) setColor(value colour.Color, original bool) {
	app.input = value.Hex()
	app.useColor(value, original)
}

// useColor makes value the current color and leaves the color field's text as
// typed; setColor also rewrites that text.
func (app *dipo) useColor(value colour.Color, original bool) {
	app.current, app.inputErr = value, nil
	app.hue, app.saturation, app.brightness = value.HSV()
	if original {
		app.original = value
	}
	app.notify()
}

func (app *dipo) save() {
	if app.store != nil {
		if err := app.store.Save(app.state); err != nil {
			app.storageErr = locales.Messages.StorageNotSaved
			slog.Error("save colors", "error", err)
		} else {
			app.storageErr = nil
		}
	}
	app.notify()
}

func (app *dipo) setTheme(theme library.Theme) {
	app.state.Theme = theme
	app.applyTheme()
	app.save()
}

// setOpenAtLogin asks the system to start Dipo at login, or no more, and reads
// back what it accepted: the system, not Dipo, keeps this setting, and may hold
// the request until the user approves it in System Settings.
func (app *dipo) setOpenAtLogin(on bool) {
	if app.setLaunchAtLogin == nil {
		return
	}
	app.openAtLogin = on
	app.notify()
	app.setLaunchAtLogin(on)
	app.refreshOpenAtLogin()
}

// refreshOpenAtLogin shows whether the system starts Dipo at login now, which
// the user can also change in System Settings.
func (app *dipo) refreshOpenAtLogin() {
	if app.launchAtLogin == nil {
		return
	}
	app.launchAtLogin(func(on bool) {
		app.openAtLogin = on
		app.notify()
	})
}

// setAutoCopy turns copying a picked color on or off.
func (app *dipo) setAutoCopy(on bool) {
	app.state.NoAutoCopy = !on
	app.save()
}

// setMenuBarVisible shows or removes the menu bar icon and remembers the
// choice. With the Dock icon gone the menu bar icon is the way back in, so it
// stays.
func (app *dipo) setMenuBarVisible(visible bool) {
	if !visible && app.state.DockHidden {
		return
	}
	app.state.MenuBarHidden = !visible
	app.showMenuBarIcon(visible)
	app.save()
}

// setDockVisible shows or hides the Dock icon and remembers the choice. With
// the menu bar icon gone the Dock icon is the way back in, so it stays.
func (app *dipo) setDockVisible(visible bool) {
	if !visible && app.state.MenuBarHidden {
		return
	}
	app.state.DockHidden = !visible
	desktop.SetDockVisible(app.scope, visible)
	app.save()
}

// showMenuBarIcon puts the menu bar icon up or takes it down; nsgo's tray has
// no hide, so showing it again makes a new one.
func (app *dipo) showMenuBarIcon(visible bool) {
	if !visible {
		app.tray.Remove()
		app.tray = nil
		return
	}
	if app.tray == nil {
		app.scope.Run(func() { app.tray = desktop.NewTray(app.scope, app.trayOptions()) })
	}
}

func (app *dipo) applyTheme() {
	app.scope.SetAppearance(appearanceOf(app.state.Theme))
}

func appearanceOf(theme library.Theme) core.Appearance {
	switch theme {
	case library.ThemeLight:
		return core.AppearanceLight
	case library.ThemeDark:
		return core.AppearanceDark
	default:
		return core.AppearanceSystem
	}
}

func (app *dipo) setLanguage(language library.Language) {
	app.state.Language = language
	app.applyLanguage()
	app.menus()
	app.save()
}

func (app *dipo) applyLanguage() {
	if app.scope != nil {
		app.scope.SetLocale(string(app.state.Language))
	}
}

func (app *dipo) messages() locales.Messages {
	return locales.In(app.locale())
}

func (app *dipo) locale() string {
	if app.state.Language != library.LanguageSystem {
		return string(app.state.Language)
	}
	if app.scope != nil {
		return app.scope.Env().Locale()
	}
	return ""
}

func (app *dipo) setStatus(text localizedText, failed bool) {
	app.status, app.statusFailed, app.copyNotice = text, failed, false
	app.notify()
}

func (app *dipo) copyColor(value colour.Color) bool {
	if app.writeClipboard == nil {
		app.setStatus(locales.Messages.ClipboardNotReady, true)
		return false
	}
	if err := app.writeClipboard(value.Text(app.state.Format)); err != nil {
		app.setStatus(locales.Messages.CopyFailed, true)
		slog.Error("copy color", "error", err)
		return false
	}
	format := string(app.state.Format)
	app.setStatus(func(messages locales.Messages) string { return messages.CopiedFormat(format) }, false)
	app.copyNotice = true
	if app.stopCopyFeedback != nil {
		app.stopCopyFeedback()
	}
	if app.scope != nil {
		app.stopCopyFeedback = app.scope.AfterFunc(1500*time.Millisecond, func() {
			if app.copyNotice {
				app.setStatus(nil, false)
			}
		})
	}
	return true
}

func (app *dipo) acceptPick(value colour.Color) bool {
	app.setColor(value, true)
	app.state.Picked(value)
	ok := true
	if app.state.NoAutoCopy {
		app.setStatus(locales.Messages.ColorPicked, false)
	} else {
		ok = app.copyColor(value)
	}
	app.save()
	return ok
}

func (app *dipo) show() {
	if app.window == nil || app.sampling || app.quitting {
		return
	}
	app.window.Focus()
	app.notify()
}

func (app *dipo) showSettings() {
	// A refused shortcut leaves the one in force; its message does not outlive the window.
	app.shortcutErr = nil
	app.notify()
	if app.showSettingsWindow != nil {
		app.showSettingsWindow()
	}
	app.refreshPermission()
	app.refreshOpenAtLogin()
}

// The settings window is tall enough to show every section without scrolling.
const (
	settingsWidth  = 460
	settingsHeight = 533
)

func (app *dipo) openSettingsWindow() {
	if app.settingsWindow != nil && !app.settingsWindow.Closed() {
		app.settingsWindow.Show()
		app.settingsWindow.Focus()
		return
	}
	app.scope.Run(func() {
		app.settingsWindow = desktop.OpenWindow(app.scope, desktop.WindowOptions{
			Title: app.messages().Settings(), Width: settingsWidth, Height: settingsHeight, Key: "settings",
			Constraints: desktop.WindowConstraints{NoResize: true},
		}, app.settingsView)
		app.settingsWindow.OnClosing(app.scope, func(event *desktop.WindowClosing) {
			if !app.quitting {
				event.Cancel()
				app.settingsWindow.Hide()
			}
		})
	})
	app.settingsWindow.Focus()
}

// pickDisabled reports whether a screen pick cannot start now, for the pick
// buttons' Disabled.
func (app *dipo) pickDisabled() bool {
	app.observe()
	return app.sampling || app.pick == nil
}

// requestPick starts a screen pick once the picker is wired up, for the pick
// buttons' OnPress.
func (app *dipo) requestPick() {
	if app.pick != nil {
		app.pick()
	}
}

func (app *dipo) startPick(ctx context.Context) {
	if app.sampling || app.quitting {
		return
	}
	app.sampling, app.permissionPrompt = true, false
	app.restoreMain = app.window.Visible()
	app.window.Hide()
	app.toast.Hide()
	app.notify()
	hint := app.messages().PickHint()
	app.workers.Go(func() {
		pickContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		picked, err := sampleScreen(pickContext, hint)
		app.scope.Post(func() {
			app.sampling = false
			if app.quitting {
				return
			}
			if err != nil {
				slog.Error("pick color", "error", err)
				app.setStatus(locales.Messages.PickFailed, true)
				app.show()
				return
			}
			app.finishPick(picked)
		})
	})
}

func (app *dipo) finishPick(picked sample) {
	if picked.Status == "permission" {
		app.setStatus(nil, false)
		app.permissionChecked, app.permissionGranted, app.permissionPrompt = true, false, true
		app.show()
		app.notify()
		return
	}
	if picked.Status == "cancelled" {
		if app.restoreMain {
			app.window.Show()
		}
		app.notify()
		return
	}
	app.permissionChecked, app.permissionGranted, app.permissionPrompt = true, true, false
	ok := app.acceptPick(picked.color())
	if picked.outsideSRGB() && ok {
		previousStatus := app.status
		app.status = func(messages locales.Messages) string {
			return previousStatus.in(messages) + " · " + messages.ApproximateSRGB()
		}
	}
	if app.restoreMain || !ok || app.storageErr != nil {
		app.window.Show()
	}
	if app.toast != nil {
		if err := app.toast.ShowAt(core.Bounds{Origin: core.Point{X: core.Points(picked.X) + 12, Y: core.Points(picked.Y) + 12}}, core.PlaceBottom); err != nil {
			slog.Warn("place copy feedback", "error", err)
		}
		if app.stopToast != nil {
			app.stopToast()
		}
		app.stopToast = app.scope.AfterFunc(1400*time.Millisecond, app.toast.Hide)
	}
	app.notify()
}

func (app *dipo) refreshPermission() {
	if app.scope == nil || app.permissionChecking || app.quitting {
		return
	}
	app.permissionChecking = true
	app.notify()
	permissionContext, cancel := context.WithTimeout(app.scope.Context(), 5*time.Second)
	app.workers.Go(func() {
		defer cancel()
		granted, err := screenCapturePermission(permissionContext)
		app.scope.Post(func() {
			app.permissionChecking, app.permissionChecked = false, true
			app.permissionGranted = err == nil && granted
			if app.permissionGranted {
				app.permissionPrompt = false
			}
			if err != nil {
				app.permissionError = locales.Messages.PermissionCheckFailed
				slog.Warn("check screen recording permission", "error", err)
			} else {
				app.permissionError = nil
			}
			app.notify()
		})
	})
}

func (app *dipo) load() {
	configPath, err := os.UserConfigDir()
	if err != nil {
		app.storageErr = locales.Messages.DataFolderUnavailable
		slog.Error("locate color data", "error", err)
		return
	}
	app.store, app.state, err = library.Open(filepath.Join(configPath, "Dipo", "colors.json"))
	if err != nil {
		app.storageErr = locales.Messages.ColorFileUnreadable
		slog.Error("load colors", "error", err)
	}
	app.notify()
}

// recordShortcut takes the combination pressed while the shortcut control has
// the keyboard focus as the new pick shortcut, and says whether it consumed the
// press. A modifier pressed alone waits for its key, and Tab, Shift+Tab and an
// unmodified Escape, Return or Space keep their usual jobs, so the control can
// be left from the keyboard.
func (app *dipo) recordShortcut(event core.KeyEvent) bool {
	switch event.Key {
	case "shift", "ctrl", "alt", "meta", "tab":
		return false
	case "escape", "enter", "space":
		if event.Modifiers == (core.Modifiers{}) {
			return false
		}
	}
	if event.Repeat {
		return true
	}
	key, err := normalizeShortcut(event.Shortcut())
	if err != nil {
		app.shortcutErr = locales.Messages.InvalidShortcutInput
		app.notify()
		return true
	}
	app.registerShortcut(shortcutText(key), true)
	return true
}

// shortcutText writes a normalized shortcut such as "ctrl+alt+c" the way the
// settings show it and the file keeps it, "Ctrl+Alt+C"; normalizeShortcut reads
// it back.
func shortcutText(key string) string {
	parts := strings.Split(key, "+")
	for i, part := range parts {
		if part == "mod" {
			part = "cmd"
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "+")
}

func (app *dipo) registerShortcut(shortcut string, persist bool) {
	key, err := normalizeShortcut(shortcut)
	if err != nil {
		app.shortcutErr = locales.Messages.InvalidShortcutInput
		app.notify()
		return
	}
	if app.scope == nil || app.shortcutPending {
		return
	}
	if key == app.registeredShortcut && app.hotkey != nil && app.hotkey.State() == desktop.HotKeyRegistered {
		app.shortcutErr = nil
		app.notify()
		return
	}
	app.shortcutPending = true
	candidateScope := app.scope.Child()
	previousScope := app.hotkeyScope
	var stopWatch func()
	candidateScope.Run(func() {
		candidate := desktop.RegisterHotKey(candidateScope, key, app.pick)
		stopWatch = candidateScope.Effect(func() {
			state := candidate.State()
			if state == desktop.HotKeyPending {
				return
			}
			app.scope.Post(func() {
				if stopWatch != nil {
					stopWatch()
				}
				app.shortcutPending = false
				if state != desktop.HotKeyRegistered {
					candidateScope.Dispose()
					app.shortcutErr = locales.Messages.ShortcutUnavailable
					app.notify()
					return
				}
				app.hotkeyScope, app.hotkey, app.registeredShortcut = candidateScope, candidate, key
				if previousScope != nil {
					previousScope.Dispose()
				}
				app.shortcutErr = nil
				if persist {
					app.state.Shortcut = shortcut
					app.save()
					// Leaving the recorder shows the saved shortcut in its field.
					if app.shortcutField != nil {
						app.shortcutField.Blur()
					}
				}
				app.notify()
			})
		})
	})
}

func normalizeShortcut(shortcut string) (string, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(shortcut)), "+")
	if len(parts) < 2 {
		return "", fmt.Errorf("shortcut needs a modifier")
	}
	modifiers := make(map[string]bool)
	for _, part := range parts[:len(parts)-1] {
		switch strings.TrimSpace(part) {
		case "ctrl", "control":
			modifiers["ctrl"] = true
		case "alt", "option", "opt":
			modifiers["alt"] = true
		case "shift":
			modifiers["shift"] = true
		case "super", "cmd", "command", "meta", "mod":
			modifiers["mod"] = true
		default:
			return "", fmt.Errorf("unknown shortcut modifier")
		}
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if len(key) != 1 || !(key[0] >= 'a' && key[0] <= 'z' || key[0] >= '0' && key[0] <= '9' || key == ",") {
		return "", fmt.Errorf("unsupported shortcut key")
	}
	if !modifiers["ctrl"] && !modifiers["alt"] && !modifiers["mod"] {
		return "", fmt.Errorf("shortcut needs control, option or command")
	}
	var normalized []string
	for _, modifier := range []string{"ctrl", "alt", "shift", "mod"} {
		if modifiers[modifier] {
			normalized = append(normalized, modifier)
		}
	}
	return strings.Join(append(normalized, key), "+"), nil
}

// checkForUpdates asks the updater to check now and show what it finds, for
// the application menu and the settings button.
func (app *dipo) checkForUpdates() { updater.Check(app.scope) }

func (app *dipo) menus() {
	msgs := app.messages()
	app.scope.SetApplicationMenu([]desktop.MenuItem{
		{Label: "Dipo", Items: []desktop.MenuItem{
			{Label: msgs.AboutDipo(), Role: desktop.MenuAbout},
			{Label: msgs.CheckForUpdates(), OnSelect: app.checkForUpdates},
			{Separator: true},
			{Label: msgs.SettingsMenu(), Shortcut: "mod+,", OnSelect: app.showSettings},
			{Separator: true},
			{Label: msgs.MenuServices(), Role: desktop.MenuServices},
			{Separator: true},
			{Label: msgs.HideDipo(), Role: desktop.MenuHide},
			{Label: msgs.HideOthers(), Role: desktop.MenuHideOthers},
			{Label: msgs.ShowAll(), Role: desktop.MenuShowAll},
			{Separator: true},
			{Label: msgs.Quit(), Role: desktop.MenuQuit, Shortcut: "mod+q", OnSelect: app.scope.Quit},
		}},
		{Label: msgs.MenuEdit(), Items: []desktop.MenuItem{
			{Label: msgs.MenuUndo(), Role: desktop.MenuUndo, Shortcut: "mod+z"},
			{Label: msgs.MenuRedo(), Role: desktop.MenuRedo, Shortcut: "mod+shift+z"},
			{Separator: true},
			{Label: msgs.MenuCut(), Role: desktop.MenuCut, Shortcut: "mod+x"},
			{Label: msgs.MenuCopy(), Role: desktop.MenuCopy, Shortcut: "mod+c", OnSelect: func() { app.copyColor(app.current) }},
			{Label: msgs.MenuPaste(), Role: desktop.MenuPaste, Shortcut: "mod+v"},
			{Label: msgs.MenuSelectAll(), Role: desktop.MenuSelectAll, Shortcut: "mod+a"},
		}},
		{Label: msgs.MenuWindow(), Items: []desktop.MenuItem{
			{Label: msgs.ShowMainWindow(), OnSelect: app.show},
		}},
	})
}

func (app *dipo) toastView(scope *core.Scope) core.Node {
	return core.View{
		Class: "flex-row items-center gap-3 p-3 bg-white dark:bg-neutral-900 rounded-xl border border-neutral-200 dark:border-neutral-700",
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(28), Height: core.Pt(28), CornerRadius: 6}, StyleFunc: func() core.Style {
				app.observe()
				return core.Style{Background: core.Color(uint32(app.current.R)<<24 | uint32(app.current.G)<<16 | uint32(app.current.B)<<8 | uint32(app.current.A))}
			}},
			core.View{Class: "gap-1", Children: []core.Node{
				core.Text{Class: "text-xs font-medium", Style: core.Style{FontFamily: core.FontMonospace}, ContentFunc: func() string { app.observe(); return app.current.Text(app.state.Format) }},
				core.Text{Class: "text-xs text-neutral-500", ContentFunc: func() string { app.observe(); return app.status.in(app.messages()) }},
			}},
		},
	}
}

// trayOptions describe the menu bar icon.
func (app *dipo) trayOptions() desktop.TrayOptions {
	return desktop.TrayOptions{
		Icon: trayTemplate, Template: true, Tooltip: "Dipo",
		Menu: func() []desktop.MenuItem {
			msgs := app.messages()
			shortcut, err := normalizeShortcut(app.state.Shortcut)
			if err != nil {
				shortcut = ""
			}
			return []desktop.MenuItem{
				{Label: msgs.PickColor(), Shortcut: shortcut, Disabled: app.sampling, OnSelect: app.pick},
				{Label: msgs.ShowMainWindow(), OnSelect: app.show},
				{Separator: true},
				{Label: msgs.SettingsMenu(), OnSelect: app.showSettings},
				{Separator: true}, {Label: msgs.Quit(), OnSelect: app.scope.Quit},
			}
		},
	}
}

func (app *dipo) ready(ctx context.Context) {
	app.load()
	app.applyLanguage()
	app.scope.Post(app.applyTheme)
	app.pick = func() { app.startPick(ctx) }
	app.showSettingsWindow = app.openSettingsWindow
	app.writeClipboard = func(text string) error { app.scope.SetClipboard(text); return nil }
	app.setLaunchAtLogin = func(on bool) { desktop.SetLaunchAtLogin(app.scope, on) }
	app.launchAtLogin = func(done func(on bool)) { desktop.LaunchAtLogin(app.scope, done) }
	desktop.SetShutdownMode(app.scope, desktop.ShutdownOnExplicit)
	app.window = desktop.OpenWindow(app.scope, desktop.WindowOptions{
		Title: "Dipo", Width: 500, Height: 650, Key: "main",
		// A login start leaves the window to the menu bar icon and the Dock.
		Hidden:      desktop.LaunchedAtLogin(app.scope),
		Constraints: desktop.WindowConstraints{NoResize: true},
	}, func(scope *core.Scope) core.Node {
		desktop.SetTitleBar(scope, desktop.TitleBarHidden, 32)
		return app.view(scope)
	})
	desktop.SetMainWindow(app.scope, app.window)
	app.window.OnClosing(app.scope, func(event *desktop.WindowClosing) {
		if !app.quitting {
			event.Cancel()
			app.window.Hide()
		}
	})
	app.toast = desktop.OpenPanel(app.scope, desktop.PanelOptions{
		Width: 250, Height: 64, NoActivate: true, Transparent: true, FitContent: true,
		PointerEvents: desktop.WindowPointerNone,
	}, app.toastView)
	app.toast.Hide()
	app.showMenuBarIcon(!app.state.MenuBarHidden)
	// The Dock icon shows from the start; one the user took away goes now. A
	// file that hides both icons keeps the Dock's.
	if app.state.DockHidden && !app.state.MenuBarHidden {
		desktop.SetDockVisible(app.scope, false)
	}
	app.menus()
	desktop.OnReopen(app.scope, app.show)
	app.registerShortcut(app.state.Shortcut, false)
	app.refreshPermission()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var app *dipo
	nsgo.Run(func(scope *core.Scope) {
		app = newDipo(scope)
		scope.OnQuit(func(context.Context) {
			app.quitting = true
			cancel()
		})
		app.ready(ctx)
	})
	cancel()
	if app != nil {
		app.workers.Wait()
	}
}
