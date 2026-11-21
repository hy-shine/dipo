package main

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hy-shine/dipo/internal/colour"
	"github.com/hy-shine/dipo/internal/library"
	"github.com/hy-shine/dipo/locales"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
	updaterlocales "github.com/nsgo-dev/nsgo/updater/locales"
)

func TestMain(tests *testing.M) {
	nsgotest.Main(tests)
}

func startPanel(test *testing.T, configure func(*dipo)) (*nsgotest.App, *dipo) {
	test.Helper()
	var controller *dipo
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		controller = newDipo(scope)
		controller.state.Language = library.LanguageSimplified
		controller.applyLanguage()
		controller.writeClipboard = func(text string) error {
			scope.SetClipboard(text)
			return nil
		}
		if configure != nil {
			configure(controller)
		}
		return controller.view(scope)
	}, nsgotest.Size(500, 650), nsgotest.Environment(core.Environment{
		OS: core.OSMacOS, Idiom: core.IdiomDesktop, ReduceMotion: true,
	}))
	return application, controller
}

func startSettings(test *testing.T, configure func(*dipo)) (*nsgotest.App, *dipo) {
	test.Helper()
	var controller *dipo
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		controller = newDipo(scope)
		controller.state.Language = library.LanguageSimplified
		controller.applyLanguage()
		if configure != nil {
			configure(controller)
		}
		return controller.settingsView(scope)
	}, nsgotest.Size(settingsWidth, settingsHeight), nsgotest.Environment(core.Environment{
		OS: core.OSMacOS, Idiom: core.IdiomDesktop, ReduceMotion: true,
	}))
	return application, controller
}

func replaceInput(application *nsgotest.App, selector, text string) {
	application.Clear(selector)
	application.Type(selector, text)
	application.Submit(selector)
}

func revealPanelView(test *testing.T, application *nsgotest.App, selector string) *nsgotest.View {
	test.Helper()
	view := application.Find(selector)
	scroll := application.Find("id:main-scroll")
	viewport := scroll.Bounds()
	bounds := view.Bounds()
	if bounds.Origin.Y < viewport.Origin.Y || bounds.Origin.Y+bounds.Size.Height > viewport.Origin.Y+viewport.Size.Height {
		offset := scroll.Offset
		if bounds.Origin.Y < viewport.Origin.Y {
			offset.Y += bounds.Origin.Y - viewport.Origin.Y
		} else {
			offset.Y += bounds.Origin.Y + bounds.Size.Height - viewport.Origin.Y - viewport.Size.Height + 8
		}
		application.ScrollTo("id:main-scroll", offset)
		view = application.Find(selector)
		bounds = view.Bounds()
	}
	if bounds.Origin.Y < viewport.Origin.Y-1 || bounds.Origin.Y+bounds.Size.Height > viewport.Origin.Y+viewport.Size.Height+1 {
		test.Fatalf("%s is not visible after scrolling: bounds %+v, viewport %+v", selector, bounds, viewport)
	}
	return view
}

func requirePanelViewVisible(test *testing.T, application *nsgotest.App, selector string) *nsgotest.View {
	test.Helper()
	view := application.Find(selector)
	viewport := application.Find("id:main-scroll").Bounds()
	bounds := view.Bounds()
	if bounds.Size.Width <= 0 || bounds.Size.Height <= 0 || bounds.Origin.Y < viewport.Origin.Y-1 ||
		bounds.Origin.Y+bounds.Size.Height > viewport.Origin.Y+viewport.Size.Height+1 {
		test.Fatalf("%s is outside the default visible viewport: bounds %+v, viewport %+v", selector, bounds, viewport)
	}
	return view
}

func TestPanelInputAndCopy(test *testing.T) {
	application, controller := startPanel(test, nil)
	replaceInput(application, "id:color-input", "rgb(19 22 27)")
	if controller.current.Hex() != "#13161B" || len(controller.state.Recent) != 0 {
		test.Fatalf("input = %s, recent = %v; want #13161B without history", controller.current.Hex(), controller.state.Recent)
	}
	application.Press("id:copy-color")
	if application.Clipboard != "#13161B" {
		test.Fatalf("clipboard = %q; want #13161B", application.Clipboard)
	}
	application.Press("id:copy-format")
	application.Press("id:copy-format-1")
	application.Press("id:copy-color")
	if application.Clipboard != "rgb(19 22 27)" || len(controller.state.Recent) != 0 {
		test.Fatalf("format copy = %q, recent = %v", application.Clipboard, controller.state.Recent)
	}
	application.Press("id:favorite-current")
	if !controller.state.Favorite(controller.current) {
		test.Fatal("favorite action did not preserve current color")
	}
}

func TestPanelInputFocusSurvivesTyping(test *testing.T) {
	application, controller := startPanel(test, nil)
	application.Clear("id:color-input")
	application.Type("id:color-input", "#A1")
	if !application.Find("id:color-input").Focused {
		test.Fatal("typing lost input focus after the first reactive update")
	}
	application.Type("id:color-input", "B2C3")
	application.Submit("id:color-input")
	if controller.current.Hex() != "#A1B2C3" {
		test.Fatalf("continued input = %s; want #A1B2C3", controller.current.Hex())
	}
}

func TestConfirmedPickAndHistoryReuse(test *testing.T) {
	value := colour.Opaque(19, 22, 27)
	application, controller := startPanel(test, func(controller *dipo) {
		controller.state.Picked(value)
		controller.setColor(colour.Opaque(255, 0, 0), true)
	})
	application.Press("id:page-recent")
	application.Press("id:recent-13161B")
	if controller.current != value || application.Clipboard != value.Hex() || len(controller.state.Recent) != 1 {
		test.Fatalf("history reuse: current %v, clipboard %q, recent %v", controller.current, application.Clipboard, controller.state.Recent)
	}
	application.Post(func() { controller.acceptPick(colour.Opaque(255, 255, 255)) })
	application.Settle()
	if application.Clipboard != "#FFFFFF" || len(controller.state.Recent) != 2 {
		test.Fatalf("confirmed pick: clipboard %q, recent %v", application.Clipboard, controller.state.Recent)
	}
	application.Post(func() {
		controller.writeClipboard = func(string) error { return errors.New("clipboard unavailable") }
		controller.acceptPick(colour.Opaque(0, 0, 0))
	})
	application.Settle()
	if controller.status.in(controller.messages()) != "复制失败，请重试。" || len(controller.state.Recent) != 3 || application.Clipboard != "#FFFFFF" {
		test.Fatalf("copy failure: status %q, clipboard %q, recent %v", controller.status.in(controller.messages()), application.Clipboard, controller.state.Recent)
	}
}

func TestPanelInvalidInputKeepsColor(test *testing.T) {
	application, controller := startPanel(test, nil)
	original := controller.current
	replaceInput(application, "id:color-input", "not a color")
	if controller.current != original || controller.inputErr == nil {
		test.Fatal("invalid input changed color or did not report its error")
	}
}

func TestPanelEditorDragAndRestore(test *testing.T) {
	application, controller := startPanel(test, nil)
	original := controller.current
	canvas := application.Find("id:color-canvas").Frame.Size
	application.DragAt("id:color-canvas", core.Point{X: canvas.Width * 0.2, Y: canvas.Height * 0.2},
		core.Point{X: canvas.Width * 0.3, Y: canvas.Height * 0.2},
		core.Point{X: canvas.Width * 0.6, Y: canvas.Height * 0.5})
	if controller.current == original || len(controller.state.Recent) != 0 {
		test.Fatalf("canvas drag: current %v, recent %v", controller.current, controller.state.Recent)
	}
	application.Focus("id:color-canvas")
	brightness := controller.brightness
	application.Key("down")
	if math.Abs(controller.brightness-(brightness-1)) > 0.01 {
		test.Fatalf("canvas keyboard brightness = %v; want %v", controller.brightness, brightness-1)
	}
	application.Press("id:restore-color")
	if controller.current != original || len(controller.state.Recent) != 0 {
		test.Fatal("restore did not restore color independently of history")
	}
}

func TestPanelTypingAppliesColorWithoutReturn(test *testing.T) {
	application, controller := startPanel(test, nil)
	application.Clear("id:color-input")
	application.Type("id:color-input", "rgb(255 0 0)")
	red := colour.Opaque(255, 0, 0)
	if controller.current != red || controller.input != "rgb(255 0 0)" || controller.original != red || controller.inputErr != nil {
		test.Fatalf("typed rgb(): current %v, original %v, input %q, error %v; want red with the typed text kept", controller.current, controller.original, controller.input, controller.inputErr)
	}
	application.Press("id:copy-color")
	if application.Clipboard != "#FF0000" || len(controller.state.Recent) != 0 {
		test.Fatalf("copy after typing = %q, recent %v; want #FF0000 and no history", application.Clipboard, controller.state.Recent)
	}
	application.Clear("id:color-input")
	application.Type("id:color-input", "#12")
	if controller.current != red || controller.inputErr != nil || application.Exists("id:color-input-error") {
		test.Fatal("a half-typed color changed the color or showed an error before the field lost focus")
	}
	application.Focus("id:favorite-current")
	if controller.current != red || controller.inputErr == nil {
		test.Fatal("leaving the field with an invalid text did not report it")
	}
	application.Clear("id:color-input")
	application.Type("id:color-input", "#0f0")
	application.Focus("id:favorite-current")
	if controller.input != "#00FF00" || controller.current != colour.Opaque(0, 255, 0) || controller.inputErr != nil {
		test.Fatalf("leaving the field: input %q, current %v, error %v; want the text normalized to #00FF00", controller.input, controller.current, controller.inputErr)
	}
}

func TestPanelRestoreStripOnSwatch(test *testing.T) {
	application, controller := startPanel(test, nil)
	original := controller.current
	if application.Exists("id:restore-color") {
		test.Fatal("restore is offered before the color changed")
	}
	application.Focus("id:alpha-slider-thumb")
	application.Key("Home")
	swatch := application.Find("id:current-swatch").Bounds()
	strip := application.Find("id:restore-color").Bounds()
	if math.Abs(float64(strip.Origin.Y+strip.Size.Height-swatch.Origin.Y-swatch.Size.Height)) > 1 || math.Abs(float64(strip.Origin.X-swatch.Origin.X)) > 1 || math.Abs(float64(strip.Size.Width-swatch.Size.Width)) > 1 || strip.Size.Height >= swatch.Size.Height/2 {
		test.Fatalf("restore strip %+v is not the bottom edge of the swatch %+v", strip, swatch)
	}
	application.Press("id:restore-color")
	if controller.current != original || controller.original != original || len(controller.state.Recent) != 0 {
		test.Fatalf("restore: current %v, original %v, recent %v", controller.current, controller.original, controller.state.Recent)
	}
	if application.Exists("id:restore-color") {
		test.Fatal("restore strip stayed after the color was restored")
	}
}

func TestPanelCopyPreviewOnlyForCSSFormats(test *testing.T) {
	application, controller := startPanel(test, nil)
	if application.Exists("id:copy-preview") {
		test.Fatal("HEX shows a preview that repeats the color field")
	}
	application.Press("id:copy-format")
	application.Press("id:copy-format-1")
	if got, want := application.Find("id:copy-preview").Text, controller.current.Text(colour.RGB); got != want {
		test.Fatalf("copy preview = %q; want %q", got, want)
	}
	application.Press("id:copy-format")
	application.Press("id:copy-format-0")
	if application.Exists("id:copy-preview") {
		test.Fatal("preview stayed after switching back to HEX")
	}
}

func TestSliderBarsAlignWithNeighbours(test *testing.T) {
	application, _ := startPanel(test, nil)
	near := func(got, want core.Points) bool { return math.Abs(float64(got-want)) <= 1 }
	right := func(bounds core.Bounds) core.Points { return bounds.Origin.X + bounds.Size.Width }
	canvas, hue := application.Find("id:color-canvas").Bounds(), application.Find("id:hue-slider-bar").Bounds()
	if !near(hue.Origin.X, canvas.Origin.X) || !near(right(hue), right(canvas)) {
		test.Fatalf("hue bar %+v does not span the saturation canvas %+v", hue, canvas)
	}
	var left core.Points
	for index := range 3 {
		input := application.Find(fmt.Sprintf("id:channel-%d-input", index)).Bounds()
		bar := application.Find(fmt.Sprintf("id:channel-%d-bar", index)).Bounds()
		if !near(bar.Origin.X, input.Origin.X) || !near(right(bar), right(input)) {
			test.Fatalf("channel %d bar %+v does not span its number field %+v", index, bar, input)
		}
		if index > 0 && !near(input.Origin.X, left) {
			test.Fatalf("channel %d number field starts at %v; want %v like the first", index, input.Origin.X, left)
		}
		left = input.Origin.X
		// Bounds is the thumb's layout frame, whose left edge is its value's position; the thumb is drawn centered on it.
		thumb := fmt.Sprintf("id:channel-%d-thumb", index)
		application.Focus(thumb)
		application.Key("Home")
		if at := application.Find(thumb).Bounds(); !near(at.Origin.X-at.Size.Width/2, bar.Origin.X) {
			test.Fatalf("channel %d thumb at the minimum is drawn from %v; want the bar's start %v", index, at.Origin.X-at.Size.Width/2, bar.Origin.X)
		}
		application.Key("End")
		if at := application.Find(thumb).Bounds(); !near(at.Origin.X+at.Size.Width/2, right(bar)) {
			test.Fatalf("channel %d thumb at the maximum is drawn to %v; want the bar's end %v", index, at.Origin.X+at.Size.Width/2, right(bar))
		}
	}
}

func TestPanelAlphaAndModelsDoNotRecordHistory(test *testing.T) {
	application, controller := startPanel(test, func(controller *dipo) { controller.state.Format = colour.RGB })
	original := controller.current
	application.Focus("id:alpha-slider-thumb")
	application.Key("Home")
	if controller.current.A != 0 {
		test.Fatalf("opacity Home = %d; want 0", controller.current.A)
	}
	application.Key("End")
	if controller.current.A != 255 {
		test.Fatalf("opacity End = %d; want 255", controller.current.A)
	}
	application.Press("id:model-hsb")
	if controller.model != 1 || controller.state.Format != colour.RGB {
		test.Fatal("editing model changed the independent copy format")
	}
	replaceInput(application, "id:channel-0-input", "120")
	if controller.current.G <= controller.current.R || controller.current.G <= controller.current.B {
		test.Fatalf("HSB hue input = %v; want green-dominant color", controller.current)
	}
	application.Press("id:model-hsl")
	if controller.model != 2 {
		test.Fatal("HSL model action did not switch the editor")
	}
	application.Focus("id:channel-2-thumb")
	application.Key("End")
	if controller.current.R != 255 || controller.current.G != 255 || controller.current.B != 255 {
		test.Fatalf("HSL lightness End = %v; want white", controller.current)
	}
	application.Press("id:restore-color")
	if controller.current != original || len(controller.state.Recent) != 0 {
		test.Fatal("editing models, opacity or restore changed history/original")
	}
}

func TestColorSliderDragAndKeyboard(test *testing.T) {
	application, controller := startPanel(test, nil)
	width := application.Find("id:channel-0").Frame.Size.Width
	application.Drag("id:channel-0", core.Point{X: width * 0.4})
	if controller.current.R <= 100 {
		test.Fatalf("RGB slider drag = %d; want an increased red channel", controller.current.R)
	}
	application.Focus("id:channel-0-thumb")
	application.Key("Home")
	application.Key("right")
	if controller.current.R == 0 {
		test.Fatal("RGB slider keyboard did not change the channel")
	}
	application.Key("End")
	if controller.current.R != 255 || len(controller.state.Recent) != 0 {
		test.Fatalf("RGB slider End = %v, history = %v", controller.current, controller.state.Recent)
	}
}

func TestPanelContrastTransparencyAndSwap(test *testing.T) {
	application, controller := startPanel(test, func(controller *dipo) { controller.setColor(colour.Opaque(0, 0, 0), true) })
	application.Press("id:page-contrast")
	application.Find("text:21.00")
	requirePanelViewVisible(test, application, "id:contrast-ratio")
	requirePanelViewVisible(test, application, "id:contrast-result")
	if application.Exists("id:contrast-background-error") {
		test.Fatal("empty contrast background error occupies a layout row")
	}
	application.Press("id:contrast-swap")
	if controller.current != colour.Opaque(255, 255, 255) || controller.background != colour.Opaque(0, 0, 0) {
		test.Fatal("contrast swap did not exchange foreground and background")
	}
	replaceInput(application, "id:contrast-background", "#FFFFFF80")
	application.Find("text:背景含透明度")
	if application.Exists("id:contrast-result") || len(controller.state.Recent) != 0 {
		test.Fatal("transparent background reports a pass or contrast added history")
	}
	application.Press("id:page-recent")
	if application.Exists("id:contrast-background") {
		test.Fatal("contrast controls remained visible on the recent page")
	}
}

func TestContrastBackgroundAppliesOnBlur(test *testing.T) {
	application, controller := startPanel(test, nil)
	application.Clear("id:contrast-background")
	application.Type("id:contrast-background", "rgb(255 0 0)")
	if controller.background != colour.Opaque(255, 255, 255) {
		test.Fatal("background changed before editing finished")
	}
	application.Focus("id:color-input")
	if controller.background != colour.Opaque(255, 0, 0) || controller.backgroundInput != "#FF0000" || controller.backgroundErr != nil {
		test.Fatalf("background after blur = %v, input %q, error %q", controller.background, controller.backgroundInput, controller.backgroundErr.in(controller.messages()))
	}
}

func TestContrastInvalidBackgroundPreservesPreview(test *testing.T) {
	application, controller := startPanel(test, nil)
	original := controller.background
	ratio := application.Find("id:contrast-ratio").Text
	application.Clear("id:contrast-background")
	application.Type("id:contrast-background", "invalid")
	application.Focus("id:color-input")
	if controller.background != original || controller.backgroundErr == nil || application.Find("id:contrast-ratio").Text != ratio {
		test.Fatal("invalid background replaced the committed color or produced no feedback")
	}
	application.Clear("id:contrast-background")
	application.Type("id:contrast-background", "#000")
	if controller.backgroundErr != nil || application.Exists("id:contrast-background-error") {
		test.Fatal("editing a correction kept the previous validation error")
	}
	application.Submit("id:contrast-background")
	if controller.background != colour.Opaque(0, 0, 0) || controller.backgroundInput != "#000000" {
		test.Fatal("Return did not apply the corrected color")
	}
}

func TestContrastSwapAppliesPendingBackground(test *testing.T) {
	application, controller := startPanel(test, nil)
	original := controller.current
	application.Clear("id:contrast-background")
	application.Type("id:contrast-background", "#f00")
	application.Press("id:contrast-swap")
	if controller.current != colour.Opaque(255, 0, 0) || controller.background != original || controller.backgroundInput != original.Hex() || len(controller.state.Recent) != 0 {
		test.Fatal("swap ignored the pending background or changed color history")
	}
	application.Clear("id:contrast-background")
	application.Type("id:contrast-background", "invalid")
	application.Press("id:contrast-swap")
	if controller.current != colour.Opaque(255, 0, 0) || controller.background != original || controller.backgroundErr == nil {
		test.Fatal("swap accepted an invalid background")
	}
}

func requireContrastControlsAligned(test *testing.T, application *nsgotest.App) {
	test.Helper()
	foreground := requirePanelViewVisible(test, application, "id:contrast-foreground").Bounds()
	background := requirePanelViewVisible(test, application, "id:contrast-background").Bounds()
	swap := requirePanelViewVisible(test, application, "id:contrast-swap").Bounds()
	if math.Abs(float64(foreground.Size.Width-background.Size.Width)) > 1 || math.Abs(float64(foreground.Origin.Y-background.Origin.Y)) > 1 {
		test.Fatalf("contrast controls are not equal and aligned: foreground %+v, background %+v", foreground, background)
	}
	if math.Abs(float64(swap.Origin.Y+swap.Size.Height/2-foreground.Origin.Y-foreground.Size.Height/2)) > 1 || swap.Origin.X < foreground.Origin.X+foreground.Size.Width || swap.Origin.X+swap.Size.Width > background.Origin.X {
		test.Fatalf("swap is not centered between the controls: %+v", swap)
	}
	for _, id := range []string{"contrast-foreground-label", "contrast-background-label"} {
		label := application.Find("id:" + id).Bounds()
		if label.Origin.Y+label.Size.Height > foreground.Origin.Y {
			test.Fatalf("%s is not above its color field", id)
		}
	}
	if application.Exists("id:contrast-apply") {
		test.Fatal("contrast still shows the redundant apply button")
	}
}

func TestContrastControlsLayout(test *testing.T) {
	application, _ := startPanel(test, nil)
	requireContrastControlsAligned(test, application)
}

func TestSettingsHasNoHistoryClear(test *testing.T) {
	application, _ := startSettings(test, nil)
	application.Find("id:language-picker")
	if application.Exists("id:history-clear") {
		test.Fatal("settings still offers clearing the color history")
	}
}

func TestSettingsOpenAtLoginFollowsTheSystem(test *testing.T) {
	enabled, accept := true, true // enabled earlier in System Settings
	application, controller := startSettings(test, func(controller *dipo) {
		controller.launchAtLogin = func(done func(bool)) { done(enabled) }
		controller.setLaunchAtLogin = func(on bool) {
			if accept {
				enabled = on
			}
		}
	})
	application.Post(controller.refreshOpenAtLogin)
	application.Settle()
	if !controller.openAtLogin {
		test.Fatal("the switch ignores a login item the system already has")
	}
	application.Press("id:open-at-login")
	if enabled || controller.openAtLogin {
		test.Fatalf("turning it off left the system at %t and the switch at %t", enabled, controller.openAtLogin)
	}
	accept = false // the system holds the request, as for approval in System Settings
	application.Press("id:open-at-login")
	if enabled || controller.openAtLogin {
		test.Fatalf("a refused request left the system at %t and the switch at %t", enabled, controller.openAtLogin)
	}
}

func TestSettingsOpenAtLoginNeedsTheSystem(test *testing.T) {
	application, _ := startSettings(test, nil)
	if application.Enabled("id:open-at-login") {
		test.Fatal("the switch is offered with no system to keep it")
	}
}

func TestSettingsMenuBarSwitchIsRemembered(test *testing.T) {
	application, controller := startSettings(test, nil)
	application.Press("id:show-in-menu-bar")
	if !controller.state.MenuBarHidden {
		test.Fatal("turning the menu bar icon off was not kept")
	}
	application.Press("id:show-in-menu-bar")
	if controller.state.MenuBarHidden {
		test.Fatal("turning the menu bar icon back on was not kept")
	}
}

func TestSettingsShortcutRecorderTakesTheKeysPressed(test *testing.T) {
	application, controller := startSettings(test, nil)
	msgs := locales.In("zh-Hans")
	application.Focus("id:shortcut-recorder")
	if !slices.Contains(application.Texts(), msgs.TypeShortcut()) {
		test.Fatalf("a focused recorder does not ask for a shortcut: %q", application.Texts())
	}
	for _, press := range []struct{ key, want string }{
		{"ctrl+alt+k", "Ctrl+Alt+K"},
		{"mod+shift+j", "Shift+Cmd+J"},
		{"alt+,", "Alt+,"},
	} {
		application.Key(press.key)
		application.Settle()
		if controller.state.Shortcut != press.want || controller.shortcutErr != nil {
			test.Fatalf("after %s the shortcut is %q, error %v; want %q", press.key, controller.state.Shortcut, controller.shortcutErr != nil, press.want)
		}
		// A finished recording leaves the field, which then shows the saved shortcut
		// without waiting for another control to take the focus.
		if controller.recording || slices.Contains(application.Texts(), msgs.TypeShortcut()) || !slices.Contains(application.Texts(), press.want) {
			test.Fatalf("after %s the field shows %q, recording %t; want %q", press.key, application.Texts(), controller.recording, press.want)
		}
		application.Focus("id:shortcut-recorder")
	}
	application.Key("tab")
	if controller.recording || !slices.Contains(application.Texts(), "Alt+,") {
		test.Fatalf("Tab did not leave the recorder, or it shows %q", application.Texts())
	}
}

func TestSettingsShortcutRecorderRefusesWhatCannotBeAShortcut(test *testing.T) {
	application, controller := startSettings(test, nil)
	want := controller.state.Shortcut
	application.Focus("id:shortcut-recorder")
	for _, key := range []string{"k", "shift+k", "f5", "ctrl+f5", "ctrl+escape", "alt+space"} {
		application.Key(key)
		application.Settle()
		if controller.shortcutErr == nil || controller.state.Shortcut != want {
			test.Fatalf("%s gave shortcut %q, error %v; want %q kept and an error", key, controller.state.Shortcut, controller.shortcutErr != nil, want)
		}
		application.Find("id:shortcut-error")
		application.Post(func() { controller.shortcutErr = nil })
		application.Settle()
	}
}

func TestMainWindowOpensSettingsWindow(test *testing.T) {
	application, controller := startPanel(test, nil)
	opened := 0
	controller.showSettingsWindow = func() { opened++ }
	application.Press("id:open-settings")
	if opened != 1 {
		test.Fatalf("open-settings requested the settings window %d times; want 1", opened)
	}
	application.Find("id:color-input")
}

func TestSettingsWindowLifecycle(test *testing.T) {
	if nsgotest.Target() != "" {
		test.Skip("the native test host does not expose desktop window management")
	}
	var controller *dipo
	application := nsgotest.StartApp(test, func(scope *core.Scope) {
		controller = newDipo(scope)
		controller.applyLanguage()
		controller.permissionChecking = true
		controller.showSettingsWindow = controller.openSettingsWindow
		controller.window = desktop.OpenWindow(scope, desktop.WindowOptions{Title: "Dipo", Width: 500, Height: 650, Key: "main"}, controller.view)
		desktop.SetMainWindow(scope, controller.window)
		controller.menus()
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, ReduceMotion: true}))
	application.Press("id:open-settings")
	if controller.settingsWindow == nil || !controller.settingsWindow.Visible() || controller.settingsWindow.Closed() {
		test.Fatalf("settings window = %p, visible = %t, closed = %t; main window = %p", controller.settingsWindow, controller.settingsWindow.Visible(), controller.settingsWindow.Closed(), controller.window)
	}
	window := controller.settingsWindow
	application.Post(func() { controller.setLanguage(library.LanguageSimplified) })
	application.Settle()
	if !window.Visible() || window.Closed() {
		test.Fatal("switching language disposed the settings window")
	}
	application.Post(window.Hide)
	application.Settle()
	application.ChooseMenu("Dipo", controller.messages().SettingsMenu())
	if controller.settingsWindow != window || !window.Visible() || window.Closed() {
		test.Fatal("settings menu did not reopen the same settings window")
	}
}

func TestPickButtonShowsLabelAndShortcut(test *testing.T) {
	picks := 0
	application, controller := startPanel(test, func(controller *dipo) { controller.pick = func() { picks++ } })
	msgs := controller.messages()
	application.Find("text:" + msgs.PickButton())
	button, input := application.Find("id:pick-screen").Bounds(), application.Find("id:color-input").Bounds()
	centre := func(bounds core.Bounds) core.Points { return bounds.Origin.Y + bounds.Size.Height/2 }
	if button.Size.Width < pickButtonWidth-1 || math.Abs(float64(button.Size.Height-ui.ButtonHeight)) > 1 {
		test.Fatalf("pick button is %vx%v; want at least %v wide and %v tall", button.Size.Width, button.Size.Height, pickButtonWidth, ui.ButtonHeight)
	}
	if math.Abs(float64(centre(button)-centre(input))) > 1 || input.Origin.X+input.Size.Width > button.Origin.X {
		test.Fatalf("pick button %+v is not beside the color input %+v on one line", button, input)
	}
	application.Press("id:pick-screen")
	if picks != 1 {
		test.Fatalf("pressing the pick button started %d picks; want 1", picks)
	}
	hovered := func() []string {
		application.Hover("id:pick-screen")
		application.Advance(time.Second)
		defer application.HoverOut()
		return application.Texts()
	}
	if want := msgs.PickFromScreen() + " · " + controller.state.Shortcut; !slices.Contains(hovered(), want) {
		test.Fatalf("pick tooltip is missing %q", want)
	}
	application.Post(func() { controller.state.Shortcut = "Ctrl+Shift+K"; controller.notify() })
	application.Settle()
	if want := msgs.PickFromScreen() + " · Ctrl+Shift+K"; !slices.Contains(hovered(), want) {
		test.Fatalf("pick tooltip does not follow a changed shortcut: missing %q", want)
	}
}

func TestIconButtonNamesAppearOnKeyboardFocus(test *testing.T) {
	application, controller := startPanel(test, func(controller *dipo) { controller.pick = func() {} })
	if slices.Contains(application.Texts(), "设置") {
		test.Fatal("settings name is visible before hovering or focusing its icon")
	}
	for _, button := range []struct{ id, name string }{
		{"pick-screen", "屏幕取色 · " + controller.state.Shortcut},
		{"favorite-current", "收藏当前颜色"},
		{"contrast-swap", "交换前景背景"},
		{"open-settings", "设置"},
	} {
		application.Key("tab")
		application.Focus("id:" + button.id)
		application.Settle()
		if !slices.Contains(application.Texts(), button.name) {
			test.Fatalf("keyboard focus on %s did not show %q", button.id, button.name)
		}
		application.Focus("id:color-input")
		application.Settle()
		if slices.Contains(application.Texts(), button.name) {
			test.Fatalf("%q remained visible after focus left its icon", button.name)
		}
	}
}

func TestSettingsWindowOutlivesOpeningControl(test *testing.T) {
	if nsgotest.Target() != "" {
		test.Skip("the native test host does not expose desktop window management")
	}
	var controller *dipo
	var controlScope *core.Scope
	application := nsgotest.StartApp(test, func(scope *core.Scope) {
		controller = newDipo(scope)
		controlScope = scope.Child()
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, ReduceMotion: true}))
	application.Post(func() { controlScope.Run(controller.openSettingsWindow) })
	application.Settle()
	window := controller.settingsWindow
	if window == nil || !window.Visible() {
		test.Fatal("settings did not open from the control scope")
	}
	application.Post(controlScope.Dispose)
	application.Settle()
	if window.Closed() || !window.Visible() {
		test.Fatal("rebuilding the opening control closed the settings window")
	}
}

func TestSettingsThemeSwitchesAppearance(test *testing.T) {
	application, controller := startSettings(test, nil)
	for _, option := range []struct {
		selector   string
		theme      library.Theme
		appearance core.Appearance
	}{
		{"theme-picker-2", library.ThemeDark, core.AppearanceDark},
		{"theme-picker-1", library.ThemeLight, core.AppearanceLight},
		{"theme-picker-0", library.ThemeSystem, core.AppearanceSystem},
	} {
		application.Press("id:theme-picker")
		application.Press("id:" + option.selector)
		if controller.state.Theme != option.theme || application.Appearance != option.appearance {
			test.Fatalf("%s: theme = %q, appearance = %v; want %q, %v", option.selector, controller.state.Theme, application.Appearance, option.theme, option.appearance)
		}
	}
}

func TestPanelCopyShortcutLeavesTextSelectionAlone(test *testing.T) {
	application, controller := startPanel(test, nil)
	copies := 0
	application.Post(func() { controller.writeClipboard = func(string) error { copies++; return nil } })
	application.Settle()
	application.Focus("id:color-input")
	application.Key("mod+a")
	application.Key("mod+c")
	if copies != 0 || application.Clipboard != controller.input {
		test.Fatalf("text copy intercepted: color copies %d, clipboard %q", copies, application.Clipboard)
	}
}

func TestPanelRecentGridAndFavoriteReuse(test *testing.T) {
	favorite := colour.Opaque(19, 22, 27)
	application, controller := startPanel(test, func(controller *dipo) {
		for index := range 20 {
			controller.state.Picked(colour.Opaque(uint8(index*10), uint8(index*10), uint8(index*10)))
		}
		controller.state.ToggleFavorite(favorite)
	})
	application.Press("id:page-recent")
	for _, value := range controller.state.Recent {
		view := revealPanelView(test, application, "id:recent-"+strings.TrimPrefix(value.Hex(), "#"))
		if view.Frame.Size.Width <= 0 || view.Frame.Size.Height <= 0 {
			test.Fatalf("recent color %s has an empty frame", value.Hex())
		}
	}
	application.Press("id:page-favorites")
	application.Press("id:favorites-13161B")
	if controller.current != favorite || application.Clipboard != favorite.Hex() || len(controller.state.Recent) != 20 || len(controller.state.Favorites) != 1 {
		test.Fatalf("favorite reuse changed records: current %v, clipboard %q, state %+v", controller.current, application.Clipboard, controller.state)
	}
}

func TestEmptyHistoryPickButton(test *testing.T) {
	picks := 0
	application, controller := startPanel(test, func(controller *dipo) { controller.pick = func() { picks++ } })
	application.Press("id:page-recent")
	application.Press("id:empty-pick")
	if picks != 1 || len(controller.state.Recent) != 0 {
		test.Fatal("empty-state pick action was not delegated or created a color record")
	}
}

func TestRecentPageCountAndClearRequiresConfirmation(test *testing.T) {
	value := colour.Opaque(19, 22, 27)
	application, controller := startPanel(test, func(controller *dipo) {
		controller.state.Picked(value)
		controller.state.ToggleFavorite(value)
	})
	application.Press("id:page-recent")
	application.Find("text:" + controller.messages().RecentCount(1))
	application.Press("id:recent-clear")
	if len(controller.state.Recent) != 1 || !controller.recentClearPrompt {
		test.Fatal("recent clear happened without confirmation")
	}
	application.Find("id:recent-clear-dialog")
	application.Press("id:recent-clear-cancel")
	if len(controller.state.Recent) != 1 || controller.recentClearPrompt {
		test.Fatal("cancelling recent clear changed the history")
	}
	application.Press("id:recent-clear")
	application.Press("id:recent-clear-confirm")
	if len(controller.state.Recent) != 0 || !controller.state.Favorite(value) || controller.recentClearPrompt {
		test.Fatal("confirmed recent clear did not preserve favorites")
	}
	application.Find("text:" + controller.messages().NoRecentColors())
}

func TestSettingsMovedOutOfLibraryTabs(test *testing.T) {
	application, controller := startPanel(test, nil)
	application.Press("id:page-recent")
	settings := application.Find("id:open-settings").Bounds()
	tabs := application.Find("id:library-pages").Bounds()
	if settings.Origin.Y+settings.Size.Height > tabs.Origin.Y {
		test.Fatalf("settings button is not above the library tabs: settings %+v, tabs %+v", settings, tabs)
	}
	if application.Exists("id:recent-clear") {
		test.Fatal("recent clear is visible for an empty history")
	}
	application.Post(func() {
		controller.state.Picked(colour.Opaque(1, 2, 3))
		controller.notify()
	})
	application.Settle()
	application.Find("id:recent-clear")
}

func TestSettingsLanguageSwitchRelocalizes(test *testing.T) {
	application, controller := startSettings(test, nil)
	path := filepath.Join(test.TempDir(), "colors.json")
	store, _, err := library.Open(path)
	if err != nil {
		test.Fatal(err)
	}
	application.Post(func() { controller.store = store })
	application.Settle()
	for _, choice := range []struct {
		option   string
		language library.Language
	}{
		{"language-picker-1", library.LanguageEnglish},
		{"language-picker-3", library.LanguageTraditional},
		{"language-picker-2", library.LanguageSimplified},
		{"language-picker-0", library.LanguageSystem},
	} {
		application.Press("id:language-picker")
		application.Press("id:" + choice.option)
		if controller.state.Language != choice.language {
			test.Fatalf("language = %q; want %q", controller.state.Language, choice.language)
		}
		_, loaded, err := library.Open(path)
		if err != nil {
			test.Fatal(err)
		}
		msgs := locales.In(controller.locale())
		if !application.Exists("text:"+msgs.Language()) || !application.Exists("text:"+msgs.Picking()) {
			test.Fatalf("settings labels did not change to %q", choice.language)
		}
		if application.Title != msgs.Settings() {
			test.Fatalf("settings title = %q; want %q", application.Title, msgs.Settings())
		}
		if loaded.Language != choice.language {
			test.Fatalf("saved language = %q; want %q", loaded.Language, choice.language)
		}
	}
}

func TestInitialLanguageFollowsSystem(test *testing.T) {
	for _, example := range []struct {
		locale string
		label  string
	}{
		{"en-US", "Language"},
		{"zh-CN", "语言"},
		{"zh-SG", "语言"},
		{"zh-TW", "語言"},
		{"zh-HK", "語言"},
		{"ja-JP", "Language"},
		{"fr-FR", "Language"},
	} {
		test.Run(example.locale, func(test *testing.T) {
			var controller *dipo
			application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
				controller = newDipo(scope)
				controller.applyLanguage()
				return controller.settingsView(scope)
			}, nsgotest.Size(settingsWidth, settingsHeight), nsgotest.Environment(core.Environment{
				OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: example.locale, ReduceMotion: true,
			}))
			application.Find("text:" + example.label)
			if !application.Exists("text:" + locales.In(example.locale).LanguageSystem()) {
				test.Fatal("new installation did not select the system language option")
			}
			if controller.state.Language != library.LanguageSystem || controller.messages().Language() != example.label {
				test.Fatalf("system locale %q: preference %q, language %q", example.locale, controller.state.Language, controller.messages().Language())
			}
		})
	}
}

func TestLanguageSwitchUpdatesExistingErrors(test *testing.T) {
	application, controller := startPanel(test, nil)
	replaceInput(application, "id:color-input", "invalid")
	application.Find("text:请输入 HEX、rgb() 或 hsl() 色值。")
	application.Post(func() { controller.setLanguage(library.LanguageEnglish) })
	application.Settle()
	if got := application.Find("id:color-input-error").Text; got != "Enter a HEX, rgb(), or hsl() color." {
		test.Fatalf("input error = %q; want English", got)
	}
	if application.Exists("text:请输入 HEX、rgb() 或 hsl() 色值。") {
		test.Fatal("switching language left an existing error in Simplified Chinese")
	}
}

func TestSavedLanguageOverridesSystem(test *testing.T) {
	path := filepath.Join(test.TempDir(), "colors.json")
	store, state, err := library.Open(path)
	if err != nil {
		test.Fatal(err)
	}
	state.Language = library.LanguageEnglish
	if err := store.Save(state); err != nil {
		test.Fatal(err)
	}
	store, loaded, err := library.Open(path)
	if err != nil {
		test.Fatal(err)
	}
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		controller := newDipo(scope)
		controller.store, controller.state = store, loaded
		controller.applyLanguage()
		return controller.settingsView(scope)
	}, nsgotest.Size(settingsWidth, settingsHeight), nsgotest.Environment(core.Environment{
		OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: "zh-CN", ReduceMotion: true,
	}))
	application.Find("text:Language")
	application.Press("id:language-picker")
	application.Press("id:language-picker-0")
	application.Find("text:语言")
	_, restored, err := library.Open(path)
	if err != nil || restored.Language != library.LanguageSystem {
		test.Fatalf("restored preference = %q, %v; want system", restored.Language, err)
	}
}

func TestPermissionPromptCloses(test *testing.T) {
	application, controller := startPanel(test, func(controller *dipo) {
		controller.permissionPrompt = true
		controller.permissionChecked = true
	})
	application.Find("text:需要屏幕录制权限")
	application.Key("tab")
	application.Focus("id:permission-close")
	application.Settle()
	if !slices.Contains(application.Texts(), "关闭权限提示") {
		test.Fatal("permission close button has no keyboard focus tooltip")
	}
	application.Press("id:permission-close")
	if controller.permissionPrompt {
		test.Fatal("permission prompt did not close")
	}
}

func TestSettingsPermissionStatus(test *testing.T) {
	application, controller := startSettings(test, func(controller *dipo) {
		controller.permissionChecked = true
	})
	application.Post(func() {
		controller.permissionGranted = true
		controller.notify()
	})
	application.Settle()
	application.Find("text:已授权")
	application.Post(func() {
		controller.permissionGranted = false
		controller.notify()
	})
	application.Settle()
	application.Find("text:未授权")
	application.Press("id:permission-settings")
	if len(application.OpenedURLs) != 1 || !strings.Contains(application.OpenedURLs[0], "Privacy_ScreenCapture") {
		test.Fatalf("permission settings URL = %v", application.OpenedURLs)
	}
}

func TestApplicationMenuUpdateFailure(test *testing.T) {
	if nsgotest.Target() != "" {
		test.Skip("requires the model updater response")
	}
	for _, language := range nativeLanguages {
		test.Run(string(language), func(test *testing.T) {
			application, controller := startPanel(test, func(controller *dipo) {
				controller.state.Language = language
				controller.applyLanguage()
			})
			application.Update = func(bool) core.UpdateCheck {
				return core.UpdateCheck{Err: errors.New("no update source")}
			}
			application.Post(controller.menus)
			application.Settle()
			application.ChooseMenu("Dipo", controller.messages().CheckForUpdates())
			if len(application.UpdateChecks) != 1 || application.UpdateChecks[0] {
				test.Fatalf("update checks = %v; want one manual check", application.UpdateChecks)
			}
			alerts := application.Alerts()
			if len(alerts) != 1 || alerts[0].Title != updaterlocales.In(string(language)).UpdateError() {
				test.Fatalf("update error alerts = %+v; want one localized failure", alerts)
			}
			application.Answer(0)
		})
	}
}

func TestSettingsCheckNowChecksForUpdates(test *testing.T) {
	if nsgotest.Target() != "" {
		test.Skip("requires the model updater response")
	}
	application, _ := startSettings(test, nil)
	application.Update = func(bool) core.UpdateCheck {
		return core.UpdateCheck{Err: errors.New("no update source")}
	}
	application.Press("id:check-updates")
	if len(application.UpdateChecks) != 1 || application.UpdateChecks[0] {
		test.Fatalf("update checks = %v; want one manual check", application.UpdateChecks)
	}
	application.Answer(0)
}

func TestAboutVersionNamesTheVersion(test *testing.T) {
	msgs := locales.In("zh-Hans")
	if got := aboutVersion(msgs, "1.2.3"); got != "版本 1.2.3" {
		test.Fatalf("version line = %q; want %q", got, "版本 1.2.3")
	}
	if got := aboutVersion(msgs, ""); got != "" {
		test.Fatalf("version line = %q for a build with no version; want none", got)
	}
}

func TestMainWindowLayout(test *testing.T) {
	application, controller := startPanel(test, nil)
	if controller.page != 0 {
		test.Fatalf("default page = %d; want contrast page 0", controller.page)
	}
	requirePanelViewVisible(test, application, "id:contrast-ratio")
	requirePanelViewVisible(test, application, "id:contrast-result")
	requirePanelViewVisible(test, application, "text:"+controller.messages().ContrastCaption())
	if application.Exists("id:contrast-background-error") {
		test.Fatal("valid default background renders an empty error row")
	}
	for _, selector := range []string{"color-input", "copy-color", "pick-screen", "model-rgb", "model-hsb", "model-hsl", "color-canvas", "page-recent", "page-favorites", "page-contrast"} {
		view := revealPanelView(test, application, "id:"+selector)
		if view.Frame.Size.Width <= 0 || view.Frame.Size.Height <= 0 {
			test.Errorf("%s is not laid out: %+v", selector, view.Frame)
		}
	}
	if view := application.Find("id:open-settings"); view.Frame.Size.Width <= 0 || view.Frame.Size.Height <= 0 {
		test.Errorf("settings button is not laid out: %+v", view.Frame)
	}
	if application.Exists("text:HEX 已复制") {
		test.Fatal("main window displays a permanent copy footer")
	}
}

func TestToastShowsWhatWasCopied(test *testing.T) {
	value := colour.Opaque(255, 107, 53)
	var controller *dipo
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		controller = newDipo(scope)
		controller.state.Format = colour.RGB
		controller.setColor(value, true)
		return controller.toastView(scope)
	}, nsgotest.Size(250, 64))
	texts := application.Texts()
	if !slices.Contains(texts, value.Text(colour.RGB)) || slices.Contains(texts, value.Hex()) {
		test.Fatalf("toast texts %q; want %q and not %q", texts, value.Text(colour.RGB), value.Hex())
	}
}

func TestSettingsAutoCopySwitch(test *testing.T) {
	application, controller := startSettings(test, nil)
	if controller.state.NoAutoCopy {
		test.Fatal("copying after a pick starts out off")
	}
	application.Press("id:auto-copy")
	if !controller.state.NoAutoCopy {
		test.Fatal("turning auto copy off was not kept")
	}
	application.Press("id:auto-copy")
	if controller.state.NoAutoCopy {
		test.Fatal("turning auto copy back on was not kept")
	}
}

func TestSettingsReopenDropsShortcutError(test *testing.T) {
	application, controller := startSettings(test, nil)
	application.Post(func() {
		controller.shortcutErr = locales.Messages.ShortcutUnavailable
		controller.notify()
	})
	application.Settle()
	application.Find("id:shortcut-error")
	application.Post(controller.showSettings)
	application.Settle()
	if controller.shortcutErr != nil || slices.Contains(application.Texts(), locales.Messages.ShortcutUnavailable(locales.In("zh-Hans"))) {
		test.Fatal("a refused shortcut's message outlived the settings window")
	}
}
