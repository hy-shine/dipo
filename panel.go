package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hy-shine/dipo/internal/colour"
	"github.com/hy-shine/dipo/locales"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

func (app *dipo) view(cx *core.Scope) core.Node {
	closeWindow := func() {
		if window := desktop.CurrentWindow(cx); window != nil {
			window.Hide()
		}
	}
	cx.OnKey("escape", closeWindow)
	cx.OnKey("mod+w", closeWindow)
	return core.Dynamic(func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		return core.View{Style: core.Style{FlexGrow: 1, Padding: core.Edges{Left: 20, Right: 20, Bottom: 20}, Gap: 10}, StyleFunc: func() core.Style { return core.Style{Background: colors.Background(), Color: colors.Text()} }, Children: []core.Node{
			core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(max(32, desktop.TitleBar(cx).Top)), FlexShrink: 0}
			}, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1}},
				withTooltip(locales.For(cx).Settings(), ui.Button{TestID: "open-settings", Label: locales.For(cx).Settings(), Icon: lucide.Settings, Variant: ui.ButtonGhost, Size: ui.SizeSmall, OnPress: app.showSettings}),
			}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 10}, Children: []core.Node{
				core.Show{When: func() bool { app.observe(); return app.permissionPrompt }, Then: app.permissionBanner},
				core.Scroll{TestID: "main-scroll", Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{
					core.View{Style: core.Style{Gap: 12, Padding: core.Edges{Right: 4}}, Children: []core.Node{app.currentView(cx), app.outputView(cx), separator(cx), app.editorView(cx), separator(cx), app.libraryView(cx)}},
				}},
			}},
			core.Show{When: func() bool { app.observe(); return app.storageErr != nil }, Then: func(cx *core.Scope) core.Node {
				return core.Text{TestID: "storage-error", Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: ui.ColorsOf(cx).Warn()} }, ContentFunc: func() string { app.observe(); return app.storageErr.in(locales.For(cx)) }}
			}},
			core.Show{When: func() bool {
				app.observe()
				return app.status != nil && app.statusFailed && !app.permissionPrompt
			}, Then: func(cx *core.Scope) core.Node {
				return core.Text{TestID: "action-error", Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: ui.ColorsOf(cx).Danger()} }, ContentFunc: func() string { app.observe(); return app.status.in(locales.For(cx)) }}
			}},
		}}
	})
}

// The current-color swatch is swatchSize square. Once an edit moves the color
// off the original, a strip stripHeight tall along its bottom edge shows the
// original, and pressing the strip restores it.
const (
	swatchSize   core.Points = 52
	swatchRadius core.Points = ui.RadiusControl
	stripHeight  core.Points = 16
	// stripHoverShade is how much white the strip takes under the pointer; the
	// strip shows user colors, so a theme's hover tint would not read on it.
	stripHoverShade = 0.25
)

// The pick button is a tonal accent button as ui.Badge's accent variant is,
// which ui.Button has no variant for: a wash of the accent over whatever is
// behind it, a rim of it, and text that leans to the text color to read in
// both appearances. It is as wide as pickButtonWidth at least, so the input
// beside it keeps its width in every language.
const (
	pickButtonWidth core.Points = 64
	pickWash                    = 0.12
	pickWashHover               = 0.2
	pickWashPressed             = 0.28
	pickRim                     = 0.3
	pickTextLean                = 0.3
	pickGlyphSize   core.Points = 14
)

// Transparency shows as light-gray squares on white in both appearances, so a
// translucent color reads the same in light and dark.
const (
	checkerboardBase  core.Color = 0xffffffff
	checkerboardColor core.Color = 0xccccccff
)

// Every custom corner is a continuous curve of one of nsgo's radii, as nsgo's
// own controls draw theirs: ui.RadiusControl for controls and small tiles,
// ui.RadiusCard for cards, and radiusTiny for a swatch inside a control.
const radiusTiny core.Points = 4

// rounded is a style with corners of radius, cut as a continuous curve.
func rounded(radius core.Points) core.Style {
	return core.Style{CornerRadius: radius, BorderCurve: core.BorderCurveContinuous}
}

// colorSwatch shows what read returns, a translucent color over a checkerboard
// of cell-point squares. The caller's style sizes it. Only a translucent color
// gets the white base: under an opaque one it would show as a light fringe
// along the rounded edge.
func colorSwatch(id, label string, cell core.Points, style core.Style, read func() colour.Color) core.Node {
	return base.ColorSwatch{TestID: id, Label: label, CheckerboardColor: checkerboardColor, CheckerboardSize: cell,
		Color: func() core.Color { return nsgoViewColor(read()) }, Style: style,
		StyleFunc: func() core.Style {
			if read().A < 255 {
				return core.Style{Background: checkerboardBase}
			}
			return core.Style{}
		}}
}

func (app *dipo) swatchView(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	msgs := locales.For(cx)
	fill := core.Style{Width: core.Pct(100), Height: core.Pct(100)}
	return core.View{Style: core.Style{Width: core.Pt(swatchSize), Height: core.Pt(swatchSize), FlexShrink: 0}, Children: []core.Node{
		colorSwatch("current-swatch", msgs.CurrentColorPreview(), 8, rounded(swatchRadius).Merge(core.Style{Width: core.Pt(swatchSize), Height: core.Pt(swatchSize), BorderWidth: 0.5, BorderColor: colors.Border()}),
			func() colour.Color { app.observe(); return app.current }),
		core.Show{When: func() bool { app.observe(); return app.current != app.original }, Then: func(cx *core.Scope) core.Node {
			restore := locales.For(cx).RestoreOriginal()
			return core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Bottom: core.Pt(0), Width: core.Pct(100), Height: core.Pt(stripHeight), CornerRadii: core.Corners{BottomLeft: swatchRadius, BottomRight: swatchRadius}, BorderCurve: core.BorderCurveContinuous, OverflowX: core.Hidden, OverflowY: core.Hidden}, Children: []core.Node{
				colorSwatch("original-swatch", "", 4, fill, func() colour.Color { app.observe(); return app.original }),
				core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Width: core.Pct(100), Height: core.Pt(1)}, StyleFunc: func() core.Style { return core.Style{Background: ui.ColorsOf(cx).Border()} }},
				ui.Tooltip{Text: restore, Placement: core.PlaceBottom, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Width: core.Pct(100), Height: core.Pct(100)}, Trigger: base.Button{TestID: "restore-color", Label: restore, OnPress: func(core.PressEvent) { app.setColor(app.original, false) },
					Style: fill, StyleFunc: func() core.Style {
						return core.Style{Hover: &core.Style{Background: ui.Alpha(core.RGB(0xffffff), stripHoverShade)}, FocusVisible: ui.FocusRing(cx, 0)}
					}}},
			}}
		}},
	}}
}

func (app *dipo) currentView(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	msgs := locales.For(cx)
	return core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 16, AlignItems: core.AlignCenter}, Children: []core.Node{
			app.swatchView(cx),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 8}, Children: []core.Node{
				muted(cx, msgs.CurrentColor(), 11),
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
					ui.Input{TestID: "color-input", Label: msgs.EnterColor(), Value: func() string { app.observe(); return app.input }, OnChange: app.editInput, OnSubmit: func(string) { app.applyInput() }, OnBlur: app.commitInput, Placeholder: msgs.ColorPlaceholder(), Style: core.Style{FlexGrow: 1, FlexShrink: 1, Width: core.Pt(0), FontFamily: core.FontMonospace}},
					app.pickButton(cx),
					withTooltip(msgs.FavoriteCurrent(), ui.Button{TestID: "favorite-current", Label: msgs.FavoriteCurrent(), Variant: ui.ButtonGhost, Size: ui.SizeSmall, Icon: lucide.Star, Tint: func() core.Color {
						app.observe()
						if app.state.Favorite(app.current) {
							return colors.Accent()
						}
						return colors.Text()
					}, OnPress: func() { app.state.ToggleFavorite(app.current); app.save() }}),
				}},
			}},
		}},
		core.Show{When: func() bool { app.observe(); return app.inputErr != nil }, Then: func(*core.Scope) core.Node {
			return core.Text{TestID: "color-input-error", Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Danger()} }, ContentFunc: func() string { app.observe(); return app.inputErr.in(msgs) }}
		}},
	}}
}

// pickButton starts a screen pick from the current-color row. Its tooltip names
// the global shortcut as it is set now.
func (app *dipo) pickButton(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	msgs := locales.For(cx)
	text := func() core.Color { return ui.Mix(colors.Accent(), colors.Text(), pickTextLean) }
	return ui.Tooltip{Placement: core.PlaceBottom, Style: core.Style{FlexShrink: 0},
		TextFunc: func() string {
			app.observe()
			return msgs.PickFromScreen() + " · " + app.state.Shortcut
		},
		Trigger: base.Button{TestID: "pick-screen", Label: msgs.PickFromScreen(), Disabled: app.pickDisabled, OnPress: func(core.PressEvent) { app.requestPick() },
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6, FlexShrink: 0,
				Height: core.Pt(ui.ButtonHeight), MinWidth: core.Pt(pickButtonWidth), Padding: core.Symmetric(0, 10),
				CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1,
				Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
			StyleFunc: func() core.Style {
				accent := colors.Accent()
				return core.Style{Background: ui.Alpha(accent, pickWash), BorderColor: ui.Alpha(accent, pickRim),
					Hover:        &core.Style{Background: ui.Alpha(accent, pickWashHover)},
					Active:       &core.Style{Background: ui.Alpha(accent, pickWashPressed)},
					FocusVisible: ui.FocusRing(cx, 2)}
			},
			Children: []core.Node{ui.Icon(lucide.Pipette, pickGlyphSize, text), ui.Text(msgs.PickButton(), 12, ui.Medium, text)}}}
}

// editInput follows typing in the color field. A text that parses becomes the
// current color at once, so Copy never lags behind the field; the field keeps
// what was typed until commitInput or Return normalizes it. A text that does
// not parse yet keeps the last color and shows no error mid-edit.
func (app *dipo) editInput(text string) {
	app.input = text
	if value, err := colour.Parse(text); err == nil {
		app.useColor(value, true)
		return
	}
	app.inputErr = nil
	app.notify()
}

// commitInput finishes an edit when the color field loses focus: it rewrites a
// valid text as HEX and reports an invalid one.
func (app *dipo) commitInput() {
	if app.input != app.current.Hex() {
		app.applyInput()
	}
}

func (app *dipo) applyInput() {
	value, err := colour.Parse(app.input)
	if err != nil {
		app.inputErr = locales.Messages.InvalidColorInput
		app.notify()
		return
	}
	app.setColor(value, true)
	app.setStatus(locales.Messages.ColorLoaded, false)
}

func (app *dipo) outputView(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	msgs := locales.For(cx)
	copied := core.NewSignal(cx, false)
	copyColor := func() {
		if app.copyColor(app.current) {
			copied.Set(true)
			cx.AfterFunc(1200*time.Millisecond, func() { copied.Set(false) })
		}
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
		muted(cx, msgs.CopyAs(), 12),
		ui.Select[colour.Format]{TestID: "copy-format", Label: msgs.CopyFormat(), Style: core.Style{Width: core.Pt(140), Height: core.Pt(28)}, Options: []ui.Option[colour.Format]{{Value: colour.HEX, Label: "HEX"}, {Value: colour.RGB, Label: "CSS RGB"}, {Value: colour.HSL, Label: "CSS HSL"}, {Value: colour.P3HEX, Label: "P3 HEX"}, {Value: colour.P3, Label: "CSS P3"}}, Value: func() colour.Format { app.observe(); return app.state.Format }, OnChange: func(format colour.Format) { app.state.Format = format; app.save() }},
		// The text to copy shows beside the format, except for HEX, which the color field above already shows.
		// Two lines fit in the row's height and hold the longest, color(display-p3 …) with an alpha.
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Width: core.Pt(0)}, Children: []core.Node{
			core.Show{When: func() bool { app.observe(); return app.state.Format != colour.HEX }, Then: func(cx *core.Scope) core.Node {
				return core.Text{TestID: "copy-preview", Label: msgs.CopyPreview(), MaxLines: 2, Style: core.Style{FontFamily: core.FontMonospace, FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: ui.ColorsOf(cx).Muted()} }, Selectable: core.SelectableOn, ContentFunc: func() string { app.observe(); return app.colorText(app.current, app.state.Format) }}
			}},
		}},
		base.Button{TestID: "copy-color", Label: msgs.CopyColor(), Style: rounded(ui.RadiusControl).Merge(core.Style{Width: core.Pt(66), Height: core.Pt(ui.ButtonHeight), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, FontSize: 12, FontWeight: 500}), StyleFunc: func() core.Style {
			palette := cx.Palette()
			return core.Style{Background: palette.Accent, Color: colors.OnAccent(), Hover: &core.Style{Background: palette.AccentHover}, Active: &core.Style{Background: palette.AccentPressed}, FocusVisible: ui.FocusRing(cx, 2)}
		}, OnPress: func(core.PressEvent) { copyColor() }, Children: []core.Node{core.Text{ContentFunc: func() string {
			if copied.Get() {
				return msgs.CopiedButton()
			}
			return msgs.CopyButton()
		}}}},
	}}
}

// channelLabelWidth is the fixed width of a channel's letter, so every number
// field starts at the same x, and channelGap the space after it.
const (
	channelLabelWidth core.Points = 14
	channelGap        core.Points = 8
)

func (app *dipo) editorView(cx *core.Scope) core.Node {
	msgs := locales.For(cx)
	model := core.NewMemo(cx, func() int { app.observe(); return app.model })
	return core.View{Style: core.Style{Gap: 10}, Children: []core.Node{
		ui.Tabs[int]{TestID: "editing-model", Label: msgs.ColorModel(), Equal: true, Size: ui.SizeSmall, Style: core.Style{Width: core.Pct(100)}, Options: []ui.Option[int]{{Value: 0, Label: "RGB", TestID: "model-rgb"}, {Value: 1, Label: "HSB", TestID: "model-hsb"}, {Value: 2, Label: "HSL", TestID: "model-hsl"}}, Value: model.Get, OnChange: func(value int) { app.model = value; app.notify() }},
		core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Width: core.Pt(0), Gap: 6}, Children: []core.Node{app.saturationView(cx), app.colorSlider(cx, "hue-slider", msgs.Hue(), 359.99, 0, func() float64 { return app.hue }, func(value float64) { app.hue = value; app.fromHSV() }, func() []core.Gradient { return []core.Gradient{hueGradient()} }, false)}},
			core.Dynamic(func(cx *core.Scope) core.Node {
				selected := model.Get()
				labels, limits := [3]string{"R", "G", "B"}, [3]float64{255, 255, 255}
				if selected == 1 {
					labels, limits = [3]string{"H", "S", "B"}, [3]float64{359.99, 100, 100}
				}
				if selected == 2 {
					labels, limits = [3]string{"H", "S", "L"}, [3]float64{359.99, 100, 100}
				}
				var channels []core.Node
				for index := range 3 {
					read := func() float64 { app.observe(); return app.channelValue(index) }
					write := func(value float64) { app.changeChannel(index, value) }
					channels = append(channels, core.View{Style: core.Style{Gap: 2}, Children: []core.Node{
						core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: channelGap}, Children: []core.Node{
							core.View{Style: core.Style{Width: core.Pt(channelLabelWidth), FlexShrink: 0}, Children: []core.Node{muted(cx, labels[index], 12)}},
							ui.NumberInput{TestID: fmt.Sprintf("channel-%d-input", index), Label: msgs.ChannelValue(labels[index]), Value: read, OnChange: write, Min: 0, Max: limits[index], Step: 1, Style: core.Style{FlexGrow: 1, Width: core.Pt(112)}, IncrementLabel: msgs.Increase(), DecrementLabel: msgs.Decrease()}}},
						// The bar sits under its number field, not under the letter.
						core.View{Style: core.Style{Margin: core.Edges{Left: channelLabelWidth + channelGap}}, Children: []core.Node{
							app.colorSlider(cx, fmt.Sprintf("channel-%d", index), msgs.SliderLabel(labels[index]), limits[index], 1, read, write, func() []core.Gradient { return app.channelGradients(index) }, false),
						}},
					}})
				}
				return core.View{Style: core.Style{Width: core.Pt(170), Gap: 6}, Children: channels}
			}),
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			muted(cx, msgs.Opacity(), 12),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Width: core.Pt(0)}, Children: []core.Node{app.colorSlider(cx, "alpha-slider", msgs.Opacity(), 100, 0, func() float64 { return float64(app.current.A) / 255 * 100 }, func(value float64) {
				current := app.current
				current.A = uint8(math.Round(value / 100 * 255))
				app.setColor(current, false)
			}, func() []core.Gradient {
				opaque := app.current
				opaque.A = 255
				return []core.Gradient{core.LinearGradient(90, core.RGBA(uint32(nsgoViewColor(opaque))>>8, 0), nsgoViewColor(opaque))}
			}, true)}},
			core.Text{Style: core.Style{Width: core.Pt(40), TextAlign: core.TextAlignEnd, FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: ui.ColorsOf(cx).Muted()} }, ContentFunc: func() string { app.observe(); return fmt.Sprintf("%.0f%%", float64(app.current.A)/255*100) }},
		}},
	}}
}

func (app *dipo) fromHSV() {
	app.current = colour.FromHSV(app.hue, app.saturation, app.brightness, app.current.A)
	app.input, app.inputErr = app.current.Hex(), nil
	app.notify()
}

func (app *dipo) channelValue(index int) float64 {
	if app.model == 0 {
		return [3]float64{float64(app.current.R), float64(app.current.G), float64(app.current.B)}[index]
	}
	if app.model == 1 {
		return [3]float64{app.hue, app.saturation, app.brightness}[index]
	}
	hue, saturation, lightness := app.current.HSL()
	return [3]float64{hue, saturation, lightness}[index]
}

func (app *dipo) changeChannel(index int, value float64) {
	channels := [3]float64{app.channelValue(0), app.channelValue(1), app.channelValue(2)}
	channels[index] = value
	switch app.model {
	case 0:
		app.setColor(colour.Color{R: uint8(math.Round(channels[0])), G: uint8(math.Round(channels[1])), B: uint8(math.Round(channels[2])), A: app.current.A}, false)
	case 1:
		app.hue, app.saturation, app.brightness = channels[0], channels[1], channels[2]
		app.fromHSV()
	case 2:
		app.setColor(colour.FromHSL(channels[0], channels[1], channels[2], app.current.A), false)
	}
}

func (app *dipo) saturationView(cx *core.Scope) core.Node {
	msgs := locales.For(cx)
	var bounds core.Size
	var start core.Point
	var target core.Ref
	update := func(position core.Point) {
		if bounds.Width == 0 || bounds.Height == 0 {
			return
		}
		app.saturation = float64(max(0, min(1, position.X/bounds.Width))) * 100
		app.brightness = float64(1-max(0, min(1, position.Y/bounds.Height))) * 100
		app.fromHSV()
	}
	return core.View{TestID: "color-canvas", Label: msgs.SaturationBrightness(), AccessibilityDescription: msgs.SaturationBrightnessHint(), Ref: &target, Style: rounded(ui.RadiusControl).Merge(core.Style{Height: core.Pt(136), Overflow: core.Hidden, FocusVisible: ui.FocusRing(cx, 2)}),
		StyleFunc: func() core.Style {
			app.observe()
			hue := nsgoViewColor(colour.FromHSV(app.hue, 100, 100, 255))
			return core.Style{Gradients: []core.Gradient{core.LinearGradient(180, core.RGBA(0, 0), core.RGB(0)), core.LinearGradient(90, core.RGB(0xffffff), hue)}}
		},
		OnLayout:    func(frame core.Bounds) { bounds = frame.Size },
		OnPressDown: func(event core.PressEvent) { start = event.Position; target.Focus(); update(start) },
		OnPress: func(event core.PressEvent) {
			if !event.Keyboard {
				update(event.Position)
			}
		},
		OnPan: func(event core.PanEvent) {
			update(core.Point{X: start.X + event.Translation.X, Y: start.Y + event.Translation.Y})
		}, PanActivation: core.PanActivation{ActiveOffsetX: 1, ActiveOffsetY: 1},
		OnKeyDown: func(event core.KeyEvent) bool {
			switch event.Key {
			case "left":
				app.saturation = max(0, app.saturation-1)
			case "right":
				app.saturation = min(100, app.saturation+1)
			case "up":
				app.brightness = min(100, app.brightness+1)
			case "down":
				app.brightness = max(0, app.brightness-1)
			default:
				return false
			}
			app.fromHSV()
			return true
		}, Children: []core.Node{core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, BorderWidth: 2, BorderColor: core.RGB(0xffffff), OutlineWidth: 1, OutlineColor: core.RGB(0), Transform: core.Transform{TranslateX: -5, TranslateY: -5}}, StyleFunc: func() core.Style {
			app.observe()
			return core.Style{Left: core.Pct(float32(app.saturation)), Top: core.Pct(float32(100 - app.brightness))}
		}}},
	}
}

func (app *dipo) libraryView(cx *core.Scope) core.Node {
	msgs := locales.For(cx)
	page := core.NewMemo(cx, func() int { app.observe(); return app.page })
	return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
		ui.Tabs[int]{TestID: "library-pages", Label: msgs.ColorsAndContrast(), Equal: true, Size: ui.SizeSmall, Style: core.Style{Width: core.Pct(100)}, Options: []ui.Option[int]{{Value: 0, Label: msgs.TabContrast(), TestID: "page-contrast"}, {Value: 1, Label: msgs.TabRecent(), TestID: "page-recent"}, {Value: 2, Label: msgs.TabFavorites(), TestID: "page-favorites"}}, Value: page.Get, OnChange: func(value int) { app.page = value; app.notify() }},
		core.Dynamic(func(cx *core.Scope) core.Node {
			if page.Get() == 0 {
				return app.contrastView(cx)
			}
			favorites := page.Get() == 2
			items := core.NewMemo(cx, func() string {
				app.observe()
				values := app.state.Recent
				if favorites {
					values = app.state.Favorites
				}
				var key strings.Builder
				for _, value := range values {
					key.WriteString(value.Hex())
					key.WriteByte(';')
				}
				return key.String()
			})
			return core.Dynamic(func(cx *core.Scope) core.Node {
				items.Get()
				values, empty, prefix := app.state.Recent, msgs.NoRecentColors(), "recent-"
				if favorites {
					values, empty, prefix = app.state.Favorites, msgs.NoFavoriteColors(), "favorites-"
				}
				if len(values) == 0 {
					return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Padding: core.Symmetric(12, 0)}, Children: []core.Node{muted(cx, empty, 11), ui.Button{TestID: "empty-pick", Text: msgs.PickAColor(), Variant: ui.ButtonOutline, Size: ui.SizeSmall, Disabled: app.pickDisabled, OnPress: app.requestPick}}}
				}
				var swatches []core.Node
				for _, value := range values {
					swatches = append(swatches, core.View{TestID: prefix + strings.TrimPrefix(value.Hex(), "#"), Label: value.Hex(), Style: rounded(ui.RadiusControl).Merge(core.Style{Height: core.Pt(ui.ButtonHeight), Background: nsgoViewColor(value.Over(colour.Opaque(235, 235, 235))), BorderWidth: 0.5, BorderColor: ui.ColorsOf(cx).Border()}), OnPress: func(core.PressEvent) { app.setColor(value, true); app.copyColor(value) }})
				}
				content := []core.Node{core.View{Style: core.Style{Display: core.DisplayGrid, GridTemplateColumns: "repeat(10, minmax(0, 1fr))", Gap: 6}, Children: swatches}, muted(cx, msgs.ClickToCopy(), 11)}
				if !favorites {
					content = append([]core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Height: core.Pt(19)}, Children: []core.Node{
						core.Text{TestID: "recent-count", Content: msgs.RecentCount(len(values)), Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: ui.ColorsOf(cx).Muted()} }},
						ui.Button{TestID: "recent-clear", Text: msgs.ClearRecentShort(), Variant: ui.ButtonGhost, Size: ui.SizeSmall, OnPress: func() { app.recentClearPrompt = true; app.notify() }},
					}}}, content...)
				}
				return core.View{Style: core.Style{Gap: 8}, Children: content}
			})
		}),
		ui.AlertDialog{TestID: "recent-clear-dialog", Open: func() bool { app.observe(); return app.recentClearPrompt }, OnOpenChange: func(open bool, _ base.Reason) { app.recentClearPrompt = open; app.notify() }, Title: msgs.ClearRecentTitle(), Description: msgs.ClearRecentBody(), Actions: []ui.AlertAction{
			{Text: msgs.Cancel(), Role: core.AlertCancel, Variant: ui.ButtonOutline, TestID: "recent-clear-cancel", OnPress: func() { app.recentClearPrompt = false; app.notify() }},
			{Text: msgs.Clear(), Role: core.AlertDestructive, Color: ui.ColorsOf(cx).Danger, TestID: "recent-clear-confirm", OnPress: func() { app.state.ClearRecent(); app.recentClearPrompt = false; app.save() }},
		}},
	}}
}

func (app *dipo) contrastView(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	msgs := locales.For(cx)
	validBackground := core.NewMemo(cx, func() bool { app.observe(); return app.background.A == 255 })
	applyBackground := func() bool {
		value, err := colour.Parse(app.backgroundInput)
		if err != nil {
			app.backgroundErr = locales.Messages.InvalidBackground
			app.notify()
			return false
		}
		app.background, app.backgroundInput, app.backgroundErr = value, value.Hex(), nil
		app.notify()
		return true
	}
	return core.View{Style: core.Style{Gap: 10}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 10}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Width: core.Pt(0), Gap: 6}, Children: []core.Node{
				core.Text{TestID: "contrast-foreground-label", Content: msgs.TextColor(), Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }},
				core.View{TestID: "contrast-foreground", Style: rounded(ui.RadiusControl).Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Height: core.Pt(32), Padding: core.Symmetric(0, 8), BorderWidth: 1}), StyleFunc: func() core.Style {
					return core.Style{Background: colors.Field(), BorderColor: colors.FieldBorder()}
				}, Children: []core.Node{
					contrastColorSwatch(cx, "contrast-foreground-swatch", func() colour.Color { app.observe(); return app.current }),
					core.Text{TestID: "contrast-foreground-value", Selectable: core.SelectableOn, Style: core.Style{FontFamily: core.FontMonospace, FontSize: 13, FontWeight: 500}, ContentFunc: func() string { app.observe(); return app.current.Hex() }},
				}},
			}},
			withTooltip(msgs.SwapForegroundBackground(), ui.Button{TestID: "contrast-swap", Label: msgs.SwapForegroundBackground(), Icon: lucide.ArrowLeftRight, IconSize: 16, Style: core.Style{Width: core.Pt(28), Height: core.Pt(32)}, Variant: ui.ButtonGhost, Size: ui.SizeSmall, OnPress: func() {
				if !applyBackground() {
					return
				}
				previous := app.current
				app.setColor(app.background, true)
				app.background, app.backgroundInput, app.backgroundErr = previous, previous.Hex(), nil
				app.notify()
			}}),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Width: core.Pt(0), Gap: 6}, Children: []core.Node{
				core.Text{TestID: "contrast-background-label", Content: msgs.Background(), Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }},
				core.View{Style: core.Style{Height: core.Pt(32)}, Children: []core.Node{
					ui.Input{TestID: "contrast-background", Label: msgs.ContrastBackground(), NoAutoCorrect: true, Style: rounded(ui.RadiusControl).Merge(core.Style{Width: core.Pct(100), Height: core.Pt(32), Padding: core.Edges{Left: 32, Right: 8}, FontFamily: core.FontMonospace, FontSize: 13, FontWeight: 500}), StyleFunc: func() core.Style {
						app.observe()
						if app.backgroundErr != nil {
							return core.Style{BorderColor: colors.Danger()}
						}
						return core.Style{}
					}, Value: func() string { app.observe(); return app.backgroundInput }, Invalid: func() bool { app.observe(); return app.backgroundErr != nil }, OnChange: func(text string) { app.backgroundInput, app.backgroundErr = text, nil; app.notify() }, OnSubmit: func(string) { applyBackground() }, OnBlur: func() {
						if app.backgroundInput != app.background.Hex() {
							applyBackground()
						}
					}},
					core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(8), Top: core.Pt(8)}, Children: []core.Node{
						contrastColorSwatch(cx, "contrast-background-swatch", func() colour.Color { app.observe(); return app.background }),
					}},
				}},
			}},
		}},
		core.Show{When: func() bool { app.observe(); return app.backgroundErr != nil }, Then: func(*core.Scope) core.Node {
			return core.Text{TestID: "contrast-background-error", Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Danger()} }, ContentFunc: func() string { app.observe(); return app.backgroundErr.in(msgs) }}
		}},
		core.Show{When: validBackground.Get, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
				core.View{Style: rounded(ui.RadiusControl).Merge(core.Style{Direction: core.Row, Height: core.Pt(44), AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Padding: core.Symmetric(0, 12)}), StyleFunc: func() core.Style {
					app.observe()
					return core.Style{Background: nsgoViewColor(app.background), Color: nsgoViewColor(app.current.Over(app.background))}
				}, Children: []core.Node{core.Text{Content: "Aa", Style: core.Style{FontSize: 26, FontWeight: 600}}, core.Text{TestID: "contrast-ratio", Style: core.Style{FontSize: 15, FontFamily: core.FontMonospace}, ContentFunc: func() string {
					app.observe()
					ratio, err := colour.Contrast(app.current, app.background)
					if err != nil {
						return ""
					}
					return fmt.Sprintf("%.2f : 1", ratio)
				}}}},
				core.Text{TestID: "contrast-result", Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Muted()} }, ContentFunc: func() string {
					app.observe()
					ratio, err := colour.Contrast(app.current, app.background)
					if err != nil {
						return ""
					}
					if ratio >= 4.5 {
						return msgs.ContrastBothPass()
					}
					if ratio >= 3 {
						return msgs.ContrastNormalFails()
					}
					return msgs.ContrastBothFail()
				}},
				muted(cx, msgs.ContrastCaption(), 11),
			}}
		}, Else: func(*core.Scope) core.Node {
			return core.Text{Content: msgs.TransparentBackground(), Style: core.Style{FontSize: 11}, StyleFunc: func() core.Style { return core.Style{Color: colors.Warn()} }}
		}},
	}}
}

func contrastColorSwatch(cx *core.Scope, id string, read func() colour.Color) core.Node {
	return colorSwatch(id, "", 4, rounded(radiusTiny).Merge(core.Style{Width: core.Pt(16), Height: core.Pt(16), FlexShrink: 0, BorderWidth: 0.5, BorderColor: ui.ColorsOf(cx).Border()}), read)
}

func (app *dipo) permissionBanner(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	msgs := locales.For(cx)
	return core.View{Style: rounded(ui.RadiusCard).Merge(core.Style{Padding: core.All(12), Gap: 8, BorderWidth: 1}), StyleFunc: func() core.Style {
		return core.Style{Background: ui.Alpha(colors.Warn(), 0.08), BorderColor: ui.Alpha(colors.Warn(), 0.35)}
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween, AlignItems: core.AlignCenter}, Children: []core.Node{core.Text{Content: msgs.PermissionNeeded(), Style: core.Style{FontWeight: 600, FontSize: 12}}, withTooltip(msgs.ClosePermissionNotice(), ui.Button{TestID: "permission-close", Label: msgs.ClosePermissionNotice(), Icon: lucide.X, Variant: ui.ButtonGhost, Size: ui.SizeSmall, OnPress: func() { app.permissionPrompt = false; app.notify() }})}},
		muted(cx, msgs.PermissionNeededBody(), 11),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{ui.Button{TestID: "permission-refresh", Text: msgs.CheckAgain(), Variant: ui.ButtonOutline, Size: ui.SizeSmall, Disabled: func() bool { app.observe(); return app.permissionChecking }, OnPress: app.refreshPermission}, ui.Button{TestID: "permission-settings", Text: msgs.OpenSystemSettings(), Variant: ui.ButtonOutline, Size: ui.SizeSmall, OnPress: func() {
			cx.OpenURL("x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture", nil)
		}}}},
	}}
}

func nsgoViewColor(value colour.Color) core.Color {
	return core.Color(uint32(value.R)<<24 | uint32(value.G)<<16 | uint32(value.B)<<8 | uint32(value.A))
}

func muted(cx *core.Scope, text string, size core.Points) core.Node {
	return core.Text{Content: text, Style: core.Style{FontSize: size}, StyleFunc: func() core.Style { return core.Style{Color: ui.ColorsOf(cx).Muted()} }}
}

func withTooltip(text string, control core.Node) core.Node {
	return ui.Tooltip{Text: text, Trigger: control, Placement: core.PlaceBottom}
}

func separator(cx *core.Scope) core.Node {
	return core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: func() core.Style { return core.Style{Background: ui.ColorsOf(cx).Line()} }}
}

// A color slider is sliderHeight tall, its bar barHeight thick and its thumb
// thumbSize across.
const (
	sliderHeight core.Points = 22
	barHeight    core.Points = 8
	thumbSize    core.Points = 16
)

// colorSlider is a slider whose colored bar runs the full width of its box, so
// the bar lines up with the controls above and below it. The thumb travels
// only along the rail, a thumb radius inside each end of the bar, which keeps
// the thumb's edge flush with the bar's at both ends.
func (app *dipo) colorSlider(cx *core.Scope, id, label string, maximum, step float64, read func() float64, write func(float64), gradients func() []core.Gradient, checker bool) core.Node {
	colors := ui.ColorsOf(cx)
	bar := core.Style{Position: core.Absolute, Left: core.Pt(-thumbSize / 2), Right: core.Pt(-thumbSize / 2), Top: core.Pt(0), Bottom: core.Pt(0), CornerRadius: barHeight / 2}
	var marks []core.Node
	if checker {
		marks = append(marks, core.View{PointerEvents: core.PointerEventsNone, Style: bar.Merge(core.Style{Background: checkerboardBase, OverflowX: core.Hidden, OverflowY: core.Hidden}), Children: []core.Node{base.Checkerboard(checkerboardColor, 5)}})
	}
	marks = append(marks, core.View{TestID: id + "-bar", PointerEvents: core.PointerEventsNone, Style: bar, StyleFunc: func() core.Style { app.observe(); return core.Style{Gradients: gradients()} }})
	return base.Slider{TestID: id, ThumbTestID: id + "-thumb", Label: label, Min: func() float64 { return 0 }, Max: func() float64 { return maximum }, Step: func() float64 { return step }, Value: func() base.SliderValue { app.observe(); return base.SingleValue(read()) }, OnChange: func(value base.SliderValue) { write(value.End) },
		Style: core.Style{Height: core.Pt(sliderHeight), JustifyContent: core.JustifyCenter}, TrackStyle: core.Style{Height: core.Pt(sliderHeight), Padding: core.Edges{Left: thumbSize / 2, Right: thumbSize / 2}, JustifyContent: core.JustifyCenter}, IndicatorStyle: core.Style{Height: core.Pt(barHeight)},
		RangeStyle: core.Style{Background: core.Transparent},
		ThumbStyle: core.Style{Position: core.Absolute, Width: core.Pt(thumbSize), Height: core.Pt(thumbSize), Top: core.Pt(-(thumbSize - barHeight) / 2), CornerRadius: thumbSize / 2, Background: core.RGB(0xffffff), BorderColor: 0x00000024, BorderWidth: 0.5, ShadowColor: 0x00000040, ShadowOffset: core.Point{Y: 1}, ShadowRadius: 2, Transform: core.Transform{TranslateX: -thumbSize / 2}, FocusVisible: &core.Style{OutlineWidth: 2, OutlineOffset: 2, OutlineColor: colors.Accent()}}, Marks: marks,
	}
}

func hueGradient() core.Gradient {
	return core.LinearGradient(90, core.RGB(0xff0000), core.RGB(0xffff00), core.RGB(0x00ff00), core.RGB(0x00ffff), core.RGB(0x0000ff), core.RGB(0xff00ff), core.RGB(0xff0000))
}

func (app *dipo) channelGradients(index int) []core.Gradient {
	if app.model == 0 {
		return []core.Gradient{core.LinearGradient(90, core.RGB(0), core.RGB([3]uint32{0xff0000, 0x00ff00, 0x0000ff}[index]))}
	}
	if index == 0 {
		return []core.Gradient{hueGradient()}
	}
	var start, end colour.Color
	if app.model == 1 {
		if index == 1 {
			start, end = colour.FromHSV(app.hue, 0, app.brightness, 255), colour.FromHSV(app.hue, 100, app.brightness, 255)
		} else {
			start, end = colour.FromHSV(app.hue, app.saturation, 0, 255), colour.FromHSV(app.hue, app.saturation, 100, 255)
		}
	} else {
		hue, saturation, lightness := app.current.HSL()
		if index == 1 {
			start, end = colour.FromHSL(hue, 0, lightness, 255), colour.FromHSL(hue, 100, lightness, 255)
		} else {
			return []core.Gradient{core.LinearGradient(90, core.RGB(0), nsgoViewColor(colour.FromHSL(hue, saturation, 50, 255)), core.RGB(0xffffff))}
		}
	}
	return []core.Gradient{core.LinearGradient(90, nsgoViewColor(start), nsgoViewColor(end))}
}
