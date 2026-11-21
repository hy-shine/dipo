package main

import (
	"github.com/hy-shine/dipo/internal/library"
	"github.com/hy-shine/dipo/locales"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/ui"
)

func (app *dipo) settingsView(cx *core.Scope) core.Node {
	closeWindow := func() {
		if window := desktop.CurrentWindow(cx); window != nil {
			window.Hide()
		}
	}
	cx.OnKey("escape", closeWindow)
	cx.OnKey("mod+w", closeWindow)
	return core.Dynamic(func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		msgs := locales.For(cx)
		desktop.SetTitle(cx, msgs.Settings())
		return core.Scroll{TestID: "settings-scroll", Ref: &app.settingsScroll, Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{
			core.View{Style: core.Style{Gap: 20, Padding: core.Edges{Left: 20, Right: 20, Top: 18, Bottom: 20}}, Children: []core.Node{
				ui.FormSection{Title: msgs.General(), Children: []core.Node{
					ui.FormRow{Title: msgs.Language(), Control: ui.Select[library.Language]{TestID: "language-picker", Label: msgs.Language(), Style: core.Style{Width: core.Pt(160), Height: core.Pt(28)}, Options: []ui.Option[library.Language]{
						{Value: library.LanguageSystem, Label: msgs.LanguageSystem()},
						{Value: library.LanguageEnglish, Label: "English"},
						{Value: library.LanguageSimplified, Label: "简体中文"},
						{Value: library.LanguageTraditional, Label: "繁體中文"},
					}, Value: func() library.Language { app.observe(); return app.state.Language }, OnChange: app.setLanguage}},
					ui.FormRow{Title: msgs.Appearance(), Control: ui.Select[library.Theme]{TestID: "theme-picker", Label: msgs.Appearance(), Style: core.Style{Width: core.Pt(160), Height: core.Pt(28)}, Options: []ui.Option[library.Theme]{{Value: library.ThemeSystem, Label: msgs.ThemeSystem()}, {Value: library.ThemeLight, Label: msgs.ThemeLight()}, {Value: library.ThemeDark, Label: msgs.ThemeDark()}}, Value: func() library.Theme { app.observe(); return app.state.Theme }, OnChange: app.setTheme}},
					ui.FormRow{Title: msgs.OpenAtLogin(), Control: ui.Switch{TestID: "open-at-login", Label: msgs.OpenAtLogin(), Disabled: func() bool { return app.setLaunchAtLogin == nil }, Checked: func() bool { app.observe(); return app.openAtLogin }, OnChange: app.setOpenAtLogin}},
					ui.FormRow{Title: msgs.ShowInMenuBar(), Control: ui.Switch{TestID: "show-in-menu-bar", Label: msgs.ShowInMenuBar(), Checked: func() bool { app.observe(); return !app.state.MenuBarHidden }, OnChange: app.setMenuBarVisible}},
				}},
				ui.FormSection{Title: msgs.Picking(), Children: []core.Node{
					core.View{Children: []core.Node{
						ui.FormRow{Title: msgs.PickShortcut(), Control: app.shortcutRecorder(cx, msgs, colors)},
						core.Show{When: func() bool { app.observe(); return app.shortcutErr != nil }, Then: func(*core.Scope) core.Node {
							return core.Text{TestID: "shortcut-error", Style: core.Style{Padding: core.Edges{Left: 12, Right: 12, Bottom: 8}, FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Warn()} }, ContentFunc: func() string { app.observe(); return app.shortcutErr.in(msgs) }}
						}},
					}},
					ui.FormRow{Title: msgs.AutoCopy(), Detail: msgs.AutoCopyDetail(), Control: ui.Switch{TestID: "auto-copy", Label: msgs.AutoCopy(), Checked: func() bool { app.observe(); return !app.state.NoAutoCopy }, OnChange: app.setAutoCopy}},
					core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Gap: 12, Padding: core.Symmetric(10, 12)}, Children: []core.Node{
						core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 4}, Children: []core.Node{
							core.Text{Content: msgs.ScreenRecording(), Style: core.Style{FontSize: 13}},
							core.Text{TestID: "permission-state", Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }, ContentFunc: func() string {
								app.observe()
								switch {
								case app.permissionChecking:
									return msgs.PermissionChecking()
								case app.permissionError != nil:
									return app.permissionError.in(msgs)
								case app.permissionGranted:
									return msgs.PermissionAllowed()
								case app.permissionChecked:
									return msgs.PermissionNotAllowed()
								default:
									return msgs.PermissionUnchecked()
								}
							}},
						}},
						core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexShrink: 0}, Children: []core.Node{
							ui.Button{TestID: "permission-refresh", Text: msgs.CheckAgain(), Variant: ui.ButtonOutline, Size: ui.SizeSmall, Disabled: func() bool { app.observe(); return app.permissionChecking }, OnPress: app.refreshPermission},
							ui.Button{TestID: "permission-settings", Text: msgs.OpenSystemSettings(), Variant: ui.ButtonOutline, Size: ui.SizeSmall, OnPress: func() {
								cx.OpenURL("x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture", nil)
							}},
						}},
					}},
				}},
				ui.FormSection{Title: msgs.About(), Children: []core.Node{
					ui.FormRow{Title: "Dipo", Detail: aboutVersion(msgs, core.App().Version), Control: ui.Button{TestID: "check-updates", Text: msgs.CheckNow(), Variant: ui.ButtonOutline, Size: ui.SizeSmall, OnPress: app.checkForUpdates}},
				}},
			}},
		}}
	})
}

// shortcutRecorder is the pick shortcut's control: once it has the keyboard
// focus, by a click or by Tab, the next combination pressed becomes the
// shortcut, as in macOS's own keyboard settings. The global shortcut in force
// keeps taking its keys before the window sees them, so pressing that one here
// starts a pick instead.
func (app *dipo) shortcutRecorder(cx *core.Scope, msgs locales.Messages, colors ui.Colors) core.Node {
	var target core.Ref
	app.shortcutField = &target
	return core.View{
		TestID: "shortcut-recorder", Label: msgs.PickShortcut(), Ref: &target,
		Focusable: func() bool { return true },
		Style: core.Style{
			Width: core.Pt(140), Height: core.Pt(28), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 6, BorderWidth: 1, FocusVisible: ui.FocusRing(cx, 0),
		},
		StyleFunc: func() core.Style {
			app.observe()
			border := colors.FieldBorder()
			if app.recording {
				border = colors.Accent()
			}
			return core.Style{Background: colors.Field(), BorderColor: border}
		},
		OnPressDown: func(core.PressEvent) { target.Focus() },
		OnFocus:     func() { app.recording = true; app.notify() },
		OnBlur:      func() { app.recording = false; app.notify() },
		OnKeyDown:   app.recordShortcut,
		Children: []core.Node{core.Text{TestID: "shortcut-text", Style: core.Style{FontFamily: core.FontMonospace}, ContentFunc: func() string {
			app.observe()
			if app.recording {
				return msgs.TypeShortcut()
			}
			return app.state.Shortcut
		}}},
	}
}

// aboutVersion is the line under the app's name in settings, for the version
// nsgo.yaml sets. A build that did not come from nsgo build has none.
func aboutVersion(msgs locales.Messages, version string) string {
	if version == "" {
		return ""
	}
	return msgs.AppVersion(version)
}
