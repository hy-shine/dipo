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
	if app.acceptPick(value) || app.current != value || len(app.state.Recent) != 1 || app.status.in(app.messages()) != "复制失败，请重试。" || !app.statusFailed {
		t.Fatal("copy failure lost the confirmed color or did not report the failure")
	}
}

func TestPickCopiesInTheChosenFormat(t *testing.T) {
	app := newDipo(nil)
	app.state.Format = colour.RGB
	var clipboard string
	app.writeClipboard = func(text string) error { clipboard = text; return nil }
	value := colour.Opaque(255, 107, 53)
	if !app.acceptPick(value) || clipboard != value.Text(colour.RGB) {
		t.Fatalf("pick copied %q; want %q", clipboard, value.Text(colour.RGB))
	}
}

func TestPickWithAutoCopyOffLeavesTheClipboardAlone(t *testing.T) {
	app := newDipo(nil)
	app.state.Language = library.LanguageSimplified
	app.state.NoAutoCopy = true
	clipboard := "previous clipboard"
	app.writeClipboard = func(text string) error { clipboard = text; return nil }
	value := colour.Opaque(19, 22, 27)
	if !app.acceptPick(value) {
		t.Fatal("skipping the copy by choice was reported as a failure")
	}
	if clipboard != "previous clipboard" || app.current != value || len(app.state.Recent) != 1 || app.state.Recent[0] != value {
		t.Fatalf("clipboard %q, current %v, recent %v; want the clipboard kept and the color current and recent", clipboard, app.current, app.state.Recent)
	}
	if app.statusFailed || app.status.in(app.messages()) != "已取色" {
		t.Fatalf("status = %q, failed = %t; want a plain picked notice", app.status.in(app.messages()), app.statusFailed)
	}
}
