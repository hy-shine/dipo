package library

import (
	"slices"

	"github.com/hy-shine/dipo/internal/colour"
)

const RecentLimit = 20

type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

func (theme Theme) Valid() bool {
	return theme == ThemeSystem || theme == ThemeLight || theme == ThemeDark
}

type State struct {
	Version   int            `json:"version"`
	Format    colour.Format  `json:"format"`
	Shortcut  string         `json:"shortcut"`
	Theme     Theme          `json:"theme"`
	Recent    []colour.Color `json:"recent"`
	Favorites []colour.Color `json:"favorites"`
}

func New() State {
	return State{Version: 1, Format: colour.HEX, Shortcut: "Ctrl+Alt+C", Theme: ThemeSystem}
}

func (state *State) Picked(value colour.Color) {
	state.Recent = slices.DeleteFunc(state.Recent, func(existing colour.Color) bool { return existing == value })
	state.Recent = append([]colour.Color{value}, state.Recent...)
	if len(state.Recent) > RecentLimit {
		state.Recent = state.Recent[:RecentLimit]
	}
}

func (state *State) Favorite(value colour.Color) bool {
	return slices.Contains(state.Favorites, value)
}

func (state *State) ToggleFavorite(value colour.Color) {
	if state.Favorite(value) {
		state.Favorites = slices.DeleteFunc(state.Favorites, func(existing colour.Color) bool { return existing == value })
		return
	}
	state.Favorites = append([]colour.Color{value}, state.Favorites...)
}

func (state *State) ClearRecent() {
	state.Recent = nil
}

func (state *State) normalize() {
	if !state.Theme.Valid() {
		state.Theme = ThemeSystem
	}
	seen := make(map[colour.Color]bool)
	state.Recent = slices.DeleteFunc(state.Recent, func(value colour.Color) bool {
		duplicate := seen[value]
		seen[value] = true
		return duplicate
	})
	if len(state.Recent) > RecentLimit {
		state.Recent = state.Recent[:RecentLimit]
	}
	clear(seen)
	state.Favorites = slices.DeleteFunc(state.Favorites, func(value colour.Color) bool {
		duplicate := seen[value]
		seen[value] = true
		return duplicate
	})
}
