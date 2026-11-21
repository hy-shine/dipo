package library_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hy-shine/dipo/internal/colour"
	"github.com/hy-shine/dipo/internal/library"
)

func TestRecentAndFavorites(t *testing.T) {
	state := library.New()
	favorite := colour.Opaque(1, 2, 3)
	state.ToggleFavorite(favorite)
	for index := range 25 {
		state.Picked(colour.Opaque(uint8(index), 0, 0))
	}
	if len(state.Recent) != 20 || state.Recent[0].R != 24 || state.Recent[19].R != 5 {
		t.Fatalf("recent = %v; want latest 20 descending", state.Recent)
	}
	state.Picked(colour.Opaque(12, 0, 0))
	if len(state.Recent) != 20 || state.Recent[0].R != 12 || state.Recent[1].R != 24 {
		t.Fatalf("duplicate pick order = %v", state.Recent)
	}
	state.ClearRecent()
	if len(state.Recent) != 0 || !state.Favorite(favorite) {
		t.Fatal("clearing recent changed favorites")
	}
	state.ToggleFavorite(favorite)
	if len(state.Favorites) != 0 {
		t.Fatal("unfavorite retained color")
	}
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	store, state, err := library.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	value := colour.Color{R: 19, G: 22, B: 27, A: 128}
	state.Picked(value)
	state.ToggleFavorite(value)
	state.Format = colour.RGB
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || len(loaded.Recent) != 1 || loaded.Recent[0] != value || !loaded.Favorite(value) || loaded.Format != colour.RGB {
		t.Fatalf("reloaded = %+v, %v", loaded, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions = %v, %v", info, err)
	}
}

func TestDisplayP3FormatsPersist(t *testing.T) {
	for _, format := range []colour.Format{colour.P3HEX, colour.P3} {
		path := filepath.Join(t.TempDir(), "colors.json")
		store, state, err := library.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		state.Format = format
		if err := store.Save(state); err != nil {
			t.Fatalf("save %s: %v", format, err)
		}
		if _, loaded, err := library.Open(path); err != nil || loaded.Format != format {
			t.Fatalf("reloaded format = %s, %v; want %s", loaded.Format, err, format)
		}
	}
}

func TestThemeDefaultsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	store, state, err := library.Open(path)
	if err != nil || state.Theme != library.ThemeSystem {
		t.Fatalf("new theme = %q, %v; want system", state.Theme, err)
	}
	state.Theme = library.ThemeDark
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || loaded.Theme != library.ThemeDark {
		t.Fatalf("reloaded theme = %q, %v; want dark", loaded.Theme, err)
	}
}

func TestUnknownThemeFallsBackToSystem(t *testing.T) {
	for _, data := range []string{
		`{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C"}`,
		`{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C","theme":"sepia"}`,
	} {
		path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		_, loaded, err := library.Open(path)
		if err != nil || loaded.Theme != library.ThemeSystem {
			t.Fatalf("theme for %s = %q, %v; want system", data, loaded.Theme, err)
		}
	}
}

func TestLanguageDefaultsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	store, state, err := library.Open(path)
	if err != nil || state.Language != library.LanguageSystem {
		t.Fatalf("new language = %q, %v; want system", state.Language, err)
	}
	state.Language = library.LanguageTraditional
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || loaded.Language != library.LanguageTraditional {
		t.Fatalf("reloaded language = %q, %v; want zh-Hant", loaded.Language, err)
	}
}

func TestUnknownLanguageFallsBackToSystem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C","theme":"dark","language":"klingon"}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || loaded.Language != library.LanguageSystem || loaded.Theme != library.ThemeDark {
		t.Fatalf("language = %q, theme = %q, %v; want system and dark", loaded.Language, loaded.Theme, err)
	}
}

func TestCorruptDataIsPreserved(t *testing.T) {
	for _, original := range []string{"not JSON", `{"version":99}`, `{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C","recent":["#zzzzzz"]}`, `{"version":1,"format":"HEX","shortcut":"  "}`} {
		path := filepath.Join(t.TempDir(), "colors.json")
		if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
			t.Fatal(err)
		}
		store, state, err := library.Open(path)
		if err == nil {
			t.Fatalf("Open(%q) accepted corrupt data", original)
		}
		state.Picked(colour.Opaque(1, 2, 3))
		if err := store.Save(state); !errors.Is(err, library.ErrReadOnly) {
			t.Fatalf("save error = %v; want read-only", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original || len(state.Recent) != 1 {
			t.Fatalf("corrupt original or in-memory color changed: %s, %v", data, err)
		}
	}
}

func TestWriteFailureKeepsMemory(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "not-a-directory", "colors.json")
	store, state, err := library.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Dir(path), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	state.Picked(colour.Opaque(1, 2, 3))
	if err := store.Save(state); err == nil || len(state.Recent) != 1 {
		t.Fatalf("save = %v; recent = %v", err, state.Recent)
	}
}

func TestInvalidStateCannotReplaceReadableFile(t *testing.T) {
	for _, invalid := range []func(*library.State){
		func(state *library.State) { state.Version = 99 },
		func(state *library.State) { state.Format = "invalid" },
		func(state *library.State) { state.Shortcut = "" },
		func(state *library.State) { state.Shortcut = " \t" },
	} {
		path := filepath.Join(t.TempDir(), "colors.json")
		store, state, err := library.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		state.Picked(colour.Opaque(1, 2, 3))
		if err := store.Save(state); err != nil {
			t.Fatal(err)
		}
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		invalid(&state)
		if err := store.Save(state); !errors.Is(err, library.ErrInvalidState) {
			t.Fatalf("invalid state save = %v; want ErrInvalidState", err)
		}
		retained, err := os.ReadFile(path)
		if err != nil || string(retained) != string(original) {
			t.Fatalf("readable file changed: %s, %v", retained, err)
		}
	}
}

func TestLoadedRecordsAreNormalized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colors.json")
	state := library.New()
	for index := range 25 {
		value := colour.Opaque(uint8(index), 0, 0)
		state.Recent = append(state.Recent, value, value)
	}
	state.Favorites = []colour.Color{colour.Opaque(24, 0, 0), colour.Opaque(24, 0, 0), colour.Opaque(1, 2, 3)}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Recent) != 20 || loaded.Recent[0].R != 0 || loaded.Recent[19].R != 19 {
		t.Fatalf("loaded recent = %v; want first 20 distinct colors in file order", loaded.Recent)
	}
	if len(loaded.Favorites) != 2 || !loaded.Favorite(colour.Opaque(24, 0, 0)) {
		t.Fatalf("loaded favorites = %v; want independent distinct favorites", loaded.Favorites)
	}
	retained, err := os.ReadFile(path)
	if err != nil || string(retained) != string(data) {
		t.Fatalf("loading rewrote original data: %s, %v", retained, err)
	}
}

func TestMenuBarIconDefaultsToShownAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, state, err := library.Open(path)
	if err != nil || state.MenuBarHidden {
		t.Fatalf("menu bar icon hidden = %t, %v for a file saved before the setting; want shown", state.MenuBarHidden, err)
	}
	state.MenuBarHidden = true
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || !loaded.MenuBarHidden {
		t.Fatalf("reloaded menu bar icon hidden = %t, %v; want hidden", loaded.MenuBarHidden, err)
	}
}

func TestDockIconDefaultsToShownAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, state, err := library.Open(path)
	if err != nil || state.DockHidden {
		t.Fatalf("Dock icon hidden = %t, %v for a file saved before the setting; want shown", state.DockHidden, err)
	}
	state.DockHidden = true
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || !loaded.DockHidden {
		t.Fatalf("reloaded Dock icon hidden = %t, %v; want hidden", loaded.DockHidden, err)
	}
}

func TestAutoCopyDefaultsOnAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dipo", "colors.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"format":"HEX","shortcut":"Ctrl+Alt+C"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, state, err := library.Open(path)
	if err != nil || state.NoAutoCopy {
		t.Fatalf("auto copy off = %t, %v for a file saved before the setting; want on", state.NoAutoCopy, err)
	}
	state.NoAutoCopy = true
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := library.Open(path)
	if err != nil || !loaded.NoAutoCopy {
		t.Fatalf("reloaded auto copy off = %t, %v; want off", loaded.NoAutoCopy, err)
	}
}
