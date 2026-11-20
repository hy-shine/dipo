package library

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrReadOnly = errors.New("existing color data could not be read; preserving the original file")
var ErrInvalidState = errors.New("unsupported color data version or preferences")

type Store struct {
	path    string
	blocked bool
}

func Open(path string) (*Store, State, error) {
	store := &Store{path: path}
	state := New()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, state, nil
	}
	if err != nil {
		store.blocked = true
		return store, state, fmt.Errorf("read colors: %w", err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		store.blocked = true
		return store, state, fmt.Errorf("decode colors: %w", err)
	}
	if !loaded.validPreferences() {
		store.blocked = true
		return store, state, ErrInvalidState
	}
	loaded.normalize()
	return store, loaded, nil
}

func (store *Store) Save(state State) (saveError error) {
	if store.blocked {
		return ErrReadOnly
	}
	if !state.validPreferences() {
		return ErrInvalidState
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode colors: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("create color directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(store.path), ".colors-*.json")
	if err != nil {
		return fmt.Errorf("create color file: %w", err)
	}
	defer func() {
		if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			saveError = errors.Join(saveError, fmt.Errorf("remove temporary colors: %w", err))
		}
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return errors.Join(fmt.Errorf("write colors: %w", err), file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(fmt.Errorf("sync colors: %w", err), file.Close())
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close colors: %w", err)
	}
	if err := os.Rename(file.Name(), store.path); err != nil {
		return fmt.Errorf("replace colors: %w", err)
	}
	return nil
}

func (state *State) validPreferences() bool {
	return state.Version == 1 && state.Format.Valid() && strings.TrimSpace(state.Shortcut) != ""
}
