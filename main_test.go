package main

import (
	"errors"
	"testing"

	"github.com/hy-shine/dipo/internal/colour"
	"github.com/hy-shine/dipo/internal/library"
)

func TestShortcutCompatibility(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"Ctrl+Alt+C", "ctrl+alt+c"},
		{" Command + Shift + C ", "shift+mod+c"},
		{"option+control+7", "ctrl+alt+7"},
		{"Super+,", "mod+,"},
		{"C", ""}, {"Shift+C", ""}, {"Ctrl+", ""}, {"Ctrl+Unknown", ""}, {"bogus+c", ""},
	} {
		got, err := normalizeShortcut(test.input)
		if got != test.want || (err == nil) != (test.want != "") {
			t.Errorf("normalizeShortcut(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
}

func TestPickCancellationAndPermissionPreserveData(t *testing.T) {
	for _, status := range []string{"cancelled", "permission"} {
		t.Run(status, func(t *testing.T) {
			app := newDipo(nil)
			var clipboard = "previous clipboard"
			app.writeClipboard = func(text string) error { clipboard = text; return nil }
			original := app.current
			app.state.Picked(original)
			app.state.ToggleFavorite(original)
			app.finishPick(sample{Status: status})
			if app.current != original || clipboard != "previous clipboard" || len(app.state.Recent) != 1 || !app.state.Favorite(original) {
				t.Fatalf("%s changed data: current %v, clipboard %q, state %+v", status, app.current, clipboard, app.state)
			}
			if status == "permission" && (!app.permissionPrompt || !app.permissionChecked || app.permissionGranted) {
				t.Fatal("permission failure did not open the permission guidance")
			}
		})
	}
}

func TestCopyFailureKeepsConfirmedColor(t *testing.T) {
	app := newDipo(nil)
	app.state.Language = library.LanguageSimplified
	app.writeClipboard = func(string) error { return errors.New("clipboard unavailable") }
	value := colour.Opaque(19, 22, 27)
	if app.acceptPick(value.SRGB()) || app.current != value || len(app.state.Recent) != 1 || app.status.in(app.messages()) != "复制失败，请重试。" || !app.statusFailed {
		t.Fatal("copy failure lost the confirmed color or did not report the failure")
	}
}

func TestPickCopiesInTheChosenFormat(t *testing.T) {
	app := newDipo(nil)
	app.state.Format = colour.RGB
	var clipboard string
	app.writeClipboard = func(text string) error { clipboard = text; return nil }
	value := colour.Opaque(255, 107, 53)
	if !app.acceptPick(value.SRGB()) || clipboard != value.Text(colour.RGB) {
		t.Fatalf("pick copied %q; want %q", clipboard, value.Text(colour.RGB))
	}
}

func TestPickOutsideSRGBSaysTheCopyIsApproximate(t *testing.T) {
	for _, test := range []struct {
		name   string
		picked sample
		want   string
	}{
		{"Display P3 green", sample{Status: "picked", Red: -0.5116, Green: 1.0183, Blue: -0.3107}, "HEX copied · Approximate sRGB"},
		{"white with capture noise", sample{Status: "picked", Red: 1.0004, Green: 0.9998, Blue: 1.0015}, "HEX copied"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newDipo(nil)
			app.state.Language = library.LanguageEnglish
			app.writeClipboard = func(string) error { return nil }
			app.finishPick(test.picked)
			if got := app.status.in(app.messages()); got != test.want {
				t.Fatalf("status = %q; want %q", got, test.want)
			}
		})
	}
}

func TestDisplayP3CopiesWhatTheScreenShowed(t *testing.T) {
	app := newDipo(nil)
	app.state.Language = library.LanguageEnglish
	app.state.Format = colour.P3HEX
	var clipboard string
	app.writeClipboard = func(text string) error { clipboard = text; return nil }
	// Display P3 green, outside sRGB: its nearest sRGB color is #00FF00, whose
	// own P3 HEX is #75FB4C.
	app.finishPick(sample{Status: "picked", Red: -0.5116, Green: 1.0183, Blue: -0.3107})
	if clipboard != "#00FF00" || app.status.in(app.messages()) != "P3 HEX copied" {
		t.Fatalf("pick copied %q with status %q; want #00FF00 and no approximation", clipboard, app.status.in(app.messages()))
	}
	if app.current != colour.Opaque(0, 255, 0) || app.state.Recent[0] != colour.Opaque(0, 255, 0) {
		t.Fatalf("current %v, recent %v; want the nearest sRGB color", app.current, app.state.Recent)
	}
	translucent := app.current
	translucent.A = 128
	app.setColor(translucent, false)
	if got := app.colorText(app.current, colour.P3HEX); got != "#75FB4C80" {
		t.Fatalf("edited pick = %q; want the edited sRGB color's #75FB4C80", got)
	}
	app.setColor(app.original, false)
	app.copyColor(app.current)
	if clipboard != "#00FF00" {
		t.Fatalf("restored pick copied %q; want #00FF00", clipboard)
	}
	app.setColor(app.state.Recent[0], true)
	app.copyColor(app.current)
	if clipboard != "#75FB4C" {
		t.Fatalf("recent color copied %q; want the sRGB color's #75FB4C", clipboard)
	}
}

func TestPickOutsideDisplayP3SaysTheCopyIsApproximate(t *testing.T) {
	app := newDipo(nil)
	app.state.Language = library.LanguageEnglish
	app.state.Format = colour.P3
	var clipboard string
	app.writeClipboard = func(text string) error { clipboard = text; return nil }
	// Brighter than white, as HDR content is.
	app.finishPick(sample{Status: "picked", Red: 1.2, Green: 1.2, Blue: 1.2})
	if clipboard != "color(display-p3 1 1 1)" || app.status.in(app.messages()) != "P3 copied · Approximate Display P3" {
		t.Fatalf("pick copied %q with status %q", clipboard, app.status.in(app.messages()))
	}
}

func TestPickWithAutoCopyOffLeavesTheClipboardAlone(t *testing.T) {
	app := newDipo(nil)
	app.state.Language = library.LanguageSimplified
	app.state.NoAutoCopy = true
	clipboard := "previous clipboard"
	app.writeClipboard = func(text string) error { clipboard = text; return nil }
	value := colour.Opaque(19, 22, 27)
	if !app.acceptPick(value.SRGB()) {
		t.Fatal("skipping the copy by choice was reported as a failure")
	}
	if clipboard != "previous clipboard" || app.current != value || len(app.state.Recent) != 1 || app.state.Recent[0] != value {
		t.Fatalf("clipboard %q, current %v, recent %v; want the clipboard kept and the color current and recent", clipboard, app.current, app.state.Recent)
	}
	if app.statusFailed || app.status.in(app.messages()) != "已取色" {
		t.Fatalf("status = %q, failed = %t; want a plain picked notice", app.status.in(app.messages()), app.statusFailed)
	}
}

func TestShortcutTextReadsBackAsTheSameShortcut(t *testing.T) {
	for _, test := range []struct{ key, want string }{
		{"ctrl+alt+c", "Ctrl+Alt+C"},
		{"shift+mod+j", "Shift+Cmd+J"},
		{"ctrl+alt+shift+mod+,", "Ctrl+Alt+Shift+Cmd+,"},
		{"alt+7", "Alt+7"},
	} {
		got := shortcutText(test.key)
		if got != test.want {
			t.Errorf("shortcutText(%q) = %q; want %q", test.key, got, test.want)
		}
		if back, err := normalizeShortcut(got); err != nil || back != test.key {
			t.Errorf("normalizeShortcut(%q) = %q, %v; want %q", got, back, err, test.key)
		}
	}
}
