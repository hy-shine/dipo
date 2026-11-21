package main

import (
	"slices"
	"testing"
	"time"

	"github.com/hy-shine/dipo/internal/colour"
	"github.com/hy-shine/dipo/internal/library"
	"github.com/hy-shine/dipo/locales"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

var nativeLanguages = []library.Language{library.LanguageSimplified, library.LanguageEnglish, library.LanguageTraditional}

var nativeAppearances = []struct {
	name string
	mode core.Appearance
}{{"light", core.AppearanceLight}, {"dark", core.AppearanceDark}}

func TestNativeMainWindow(test *testing.T) {
	if nsgotest.Target() != "macos" {
		test.Skip("requires the macOS NSGo native test host")
	}
	for _, language := range nativeLanguages {
		for _, appearance := range nativeAppearances {
			test.Run(string(language)+"-"+appearance.name, func(test *testing.T) {
				var controller *dipo
				application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
					scope.SetAppearance(appearance.mode)
					desktop.SetTitleBar(scope, desktop.TitleBarHidden, 32)
					controller = newDipo(scope)
					controller.state.Language = language
					controller.applyLanguage()
					controller.permissionChecked, controller.permissionGranted = true, true
					controller.writeClipboard = func(text string) error { scope.SetClipboard(text); return nil }
					controller.pick = func() {}
					for _, value := range []colour.Color{colour.Opaque(19, 22, 27), colour.Opaque(255, 107, 53), colour.Opaque(66, 127, 220)} {
						controller.state.Picked(value)
					}
					return controller.view(scope)
				}, nsgotest.Size(500, 650))
				msgs := locales.In(string(language))
				application.Find("id:color-input")
				application.Find("id:page-contrast")
				requirePanelViewVisible(test, application, "id:contrast-ratio")
				requirePanelViewVisible(test, application, "id:contrast-result")
				requirePanelViewVisible(test, application, "text:"+msgs.ContrastCaption())
				requireContrastControlsAligned(test, application)
				if application.Exists("id:contrast-background-error") {
					test.Fatal("valid default background renders an empty error row")
				}
				for _, tip := range []struct{ id, text string }{
					{"pick-screen", msgs.PickFromScreen() + " · " + controller.state.Shortcut},
					{"favorite-current", msgs.FavoriteCurrent()},
					{"contrast-swap", msgs.SwapForegroundBackground()},
					{"open-settings", msgs.Settings()},
				} {
					application.Hover("id:" + tip.id)
					application.Advance(time.Second)
					if !slices.Contains(application.Texts(), tip.text) {
						test.Fatalf("hovering %s shows no tooltip %q", tip.id, tip.text)
					}
					if tip.id == "contrast-swap" {
						application.Screenshot("nsgo-tooltip-" + string(language) + "-" + appearance.name)
					}
					application.HoverOut()
				}
				application.Find("id:open-settings")
				application.Find("id:library-pages")
				application.Screenshot("nsgo-main-" + string(language) + "-" + appearance.name)
				application.Press("id:page-recent")
				application.Find("id:recent-13161B")
				application.Find("text:" + msgs.RecentCount(3))
				application.Screenshot("nsgo-recent-" + string(language) + "-" + appearance.name)
				application.Press("id:recent-clear")
				application.Find("id:recent-clear-dialog")
				application.Press("id:recent-clear-cancel")
				application.Find("id:recent-13161B")
			})
		}
	}
}

func TestNativeOpaqueSwatchHasNoLightFringe(test *testing.T) {
	if nsgotest.Target() != "macos" {
		test.Skip("requires the macOS NSGo native test host")
	}
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		scope.SetAppearance(core.AppearanceDark)
		controller := newDipo(scope)
		return controller.view(scope)
	}, nsgotest.Size(500, 650))
	bounds := application.Find("id:current-swatch").Bounds()
	image := application.Capture()
	scale := float64(image.Bounds().Dx()) / 500
	// The swatch's theme border and its antialiased edge, which the rounded corners carry about 2 points in, are drawn on purpose in a gray the dark theme makes lighter than the color; the light fringe this guards against shows inside them.
	const inset = 3
	for row := int((float64(bounds.Origin.Y) + inset) * scale); row < int((float64(bounds.Origin.Y+bounds.Size.Height)-inset)*scale); row++ {
		for column := int((float64(bounds.Origin.X) + inset) * scale); column < int((float64(bounds.Origin.X+bounds.Size.Width)-inset)*scale); column++ {
			red, green, blue, _ := image.At(column, row).RGBA()
			if max(red, green, blue)>>8 > 80 {
				test.Fatalf("opaque dark swatch has a bright pixel at %d,%d: %d,%d,%d", column, row, red>>8, green>>8, blue>>8)
			}
		}
	}
	application.Screenshot("nsgo-opaque-swatch-dark")
}

func TestNativeSwatchShowsOriginalInStrip(test *testing.T) {
	if nsgotest.Target() != "macos" {
		test.Skip("requires the macOS NSGo native test host")
	}
	var controller *dipo
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		scope.SetAppearance(core.AppearanceLight)
		controller = newDipo(scope)
		return controller.view(scope)
	}, nsgotest.Size(500, 650))
	application.Post(func() {
		controller.setColor(colour.Opaque(19, 22, 27), true)
		controller.setColor(colour.Opaque(255, 107, 53), false)
	})
	application.Settle()
	bounds := application.Find("id:current-swatch").Bounds()
	image := application.Capture()
	scale := float64(image.Bounds().Dx()) / 500
	sample := func(offset core.Points) (red, green, blue uint32) {
		red, green, blue, _ = image.At(int((float64(bounds.Origin.X)+float64(bounds.Size.Width)/2)*scale), int((float64(bounds.Origin.Y)+float64(offset))*scale)).RGBA()
		return red >> 8, green >> 8, blue >> 8
	}
	if red, _, blue := sample(bounds.Size.Height / 4); red < 200 || blue > 140 {
		test.Fatalf("top of the swatch = %d,_,%d; want the orange current color", red, blue)
	}
	if red, green, blue := sample(bounds.Size.Height - stripHeight/2); max(red, green, blue) > 80 {
		test.Fatalf("restore strip = %d,%d,%d; want the dark original color", red, green, blue)
	}
	application.Screenshot("nsgo-swatch-strip-light")
	application.Hover("id:restore-color")
	application.Advance(time.Second)
	if !slices.Contains(application.Texts(), controller.messages().RestoreOriginal()) {
		test.Fatalf("hovering the strip shows no tooltip %q", controller.messages().RestoreOriginal())
	}
	application.Screenshot("nsgo-swatch-strip-tooltip-light")
}

func TestNativeSettingsWindow(test *testing.T) {
	if nsgotest.Target() != "macos" {
		test.Skip("requires the macOS NSGo native test host")
	}
	for _, language := range nativeLanguages {
		for _, appearance := range nativeAppearances {
			test.Run(string(language)+"-"+appearance.name, func(test *testing.T) {
				var controller *dipo
				application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
					scope.SetAppearance(appearance.mode)
					controller = newDipo(scope)
					controller.state.Language = language
					controller.applyLanguage()
					controller.permissionChecked, controller.permissionGranted = true, true
					controller.state.Picked(colour.Opaque(19, 22, 27))
					controller.state.Picked(colour.Opaque(255, 107, 53))
					return controller.settingsView(scope)
				}, nsgotest.Size(settingsWidth, settingsHeight))
				application.Find("id:language-picker")
				application.Find("id:theme-picker")
				application.Find("id:permission-state")
				checkViewport := func(stage string, selectors []string) {
					viewport := application.Find("id:settings-scroll").Bounds()
					for _, selector := range selectors {
						bounds := application.Find("id:" + selector).Bounds()
						if bounds.Origin.X < viewport.Origin.X || bounds.Origin.Y < viewport.Origin.Y || bounds.Origin.X+bounds.Size.Width > viewport.Origin.X+viewport.Size.Width || bounds.Origin.Y+bounds.Size.Height > viewport.Origin.Y+viewport.Size.Height {
							test.Fatalf("%s is outside the settings viewport %s: bounds %+v, viewport %+v", selector, stage, bounds, viewport)
						}
					}
				}
				fitsWindow := func(stage string) {
					content, held := controller.settingsScroll.ContentBounds()
					frame, framed := controller.settingsScroll.Bounds()
					if !held || !framed || content.Size.Height > frame.Size.Height {
						test.Fatalf("the settings content is %v tall in a window showing %v, %s", content.Size.Height, frame.Size.Height, stage)
					}
				}
				rest := []string{"language-picker", "theme-picker", "shortcut-recorder", "show-in-dock", "permission-state", "permission-refresh", "permission-settings", "open-at-login", "show-in-menu-bar", "auto-copy", "check-updates"}
				checkViewport("at rest", rest)
				fitsWindow("at rest")
				application.Screenshot("nsgo-settings-" + string(language) + "-" + appearance.name)
				// Refusing a key shows its line below the field. The window scrolls to fit it, so the
				// rows further down may leave the view; the field and the line must stay in it.
				application.Focus("id:shortcut-recorder")
				application.Key("k")
				application.Settle()
				checkViewport("with the refusal line", []string{"shortcut-recorder", "shortcut-error"})
			})
		}
	}
}

func TestNativeMenuBarIconCanBeRemovedAndReturn(test *testing.T) {
	if nsgotest.Target() != "macos" {
		test.Skip("requires the macOS NSGo native test host")
	}
	var controller *dipo
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		controller = newDipo(scope)
		return controller.settingsView(scope)
	}, nsgotest.Size(settingsWidth, settingsHeight))
	application.Post(func() { controller.showMenuBarIcon(true) })
	application.Settle()
	if controller.tray == nil {
		test.Skip("the native test host has no menu bar")
	}
	application.Press("id:show-in-menu-bar")
	if controller.tray != nil || !controller.state.MenuBarHidden {
		test.Fatalf("turning the switch off left the icon %v and hidden = %t", controller.tray, controller.state.MenuBarHidden)
	}
	application.Press("id:show-in-menu-bar")
	if controller.tray == nil || controller.state.MenuBarHidden {
		test.Fatalf("turning the switch on left the icon %v and hidden = %t", controller.tray, controller.state.MenuBarHidden)
	}
}

func TestNativeSettingsLanguageSelection(test *testing.T) {
	if nsgotest.Target() != "macos" {
		test.Skip("requires the macOS NSGo native test host")
	}
	var controller *dipo
	application := nsgotest.Start(test, func(scope *core.Scope) core.Node {
		controller = newDipo(scope)
		controller.applyLanguage()
		return controller.settingsView(scope)
	}, nsgotest.Size(settingsWidth, settingsHeight))
	application.Press("id:language-picker")
	for _, option := range []string{"language-picker-0", "language-picker-1", "language-picker-2", "language-picker-3"} {
		application.Find("id:" + option)
	}
	application.Screenshot("nsgo-settings-language-options")
	for index, choice := range []struct {
		language library.Language
		option   string
	}{
		{library.LanguageEnglish, "1"},
		{library.LanguageTraditional, "3"},
		{library.LanguageSimplified, "2"},
	} {
		if index > 0 {
			application.Press("id:language-picker")
		}
		application.Press("id:language-picker-" + choice.option)
		msgs := locales.In(string(choice.language))
		if controller.state.Language != choice.language || application.Title != msgs.Settings() || !application.Exists("text:"+msgs.Language()) {
			test.Fatalf("native settings did not switch to %q: state %q, title %q", choice.language, controller.state.Language, application.Title)
		}
	}
}
