package app

import (
	"fmt"
	"go-post/internal/core"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func modLabel() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "Ctrl"
}

func searchShortcut() string {
	if runtime.GOOS == "darwin" {
		return "⌘+Space"
	}
	return "Ctrl+Space"
}

func sendShortcut() string {
	if runtime.GOOS == "darwin" {
		return "⌘ ↵"
	}
	return "Ctrl ↵"
}

func (a *app) card(c *ui.Context) ui.Element {
	p := a.palette
	return ui.Column(c).Background(p.Panel).Radius(14).Border(1, p.Border)
}

func (a *app) softCard(c *ui.Context) ui.Element {
	p := a.palette
	return ui.Column(c).Background(p.Field).Radius(12).Border(1, p.Border)
}

func (a *app) sectionLabel(c *ui.Context, text string) ui.Element {
	p := a.palette
	return ui.Text(c, strings.ToUpper(text)).FontSize(10).FontWeight(650).LetterSpacing(0.7).TextColor(p.Muted)
}

func (a *app) iconButton(c *ui.Context, name, tip string, size float32) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(size, size).Radius(8).Center().Tooltip(tip).Label(tip)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, icon(name, size)).Size(size*0.46, size*0.46).TextColor(t.TextMuted)
	})
	return b
}

func (a *app) plainButton(c *ui.Context, label, ic string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(34).Padding(0, 12).Radius(8).Gap(7).FocusRing(true).Label(label)
	b.Background(t.Surface)
	b.Border(1, t.Border)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 14)).Size(14, 14).TextColor(t.TextMuted)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(550)
	})
	return b
}

func (a *app) accentButton(c *ui.Context, label, ic string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(34).Padding(0, 14).Radius(8).Gap(7).FocusRing(true).Label(label)
	b.Background(t.Accent)
	b.TextColor(t.AccentText)
	if b.Hovered() {
		b.Background(t.AccentHover)
	}
	if b.Pressed() {
		b.Background(t.AccentPressed)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 15)).Size(14, 14).TextColor(t.AccentText)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(650).TextColor(t.AccentText)
	})
	return b
}

func (a *app) dangerButton(c *ui.Context, label, ic string) ui.Element {
	p := a.palette
	fg := contrastText(p.Bad)
	b := ui.ButtonBase(c).Height(34).Padding(0, 14).Radius(8).Gap(7).FocusRing(true).Label(label)
	b.Background(p.Bad)
	b.TextColor(fg)
	if b.Hovered() {
		b.Background(p.Bad.Mix(p.Text, 0.06))
	}
	if b.Pressed() {
		b.Background(p.Bad.Mix(ui.Hex("#000000"), 0.12))
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 15)).Size(14, 14).TextColor(fg)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(650).TextColor(fg)
	})
	return b
}

// smallButton is the compact bordered button used in a toolbar row, as the
// body's Pretty action. It is quieter than plainButton so it sits beside text.
func (a *app) smallButton(c *ui.Context, label, ic, tip string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(26).PaddingX(9).Radius(7).Gap(6).FocusRing(true).Label(label).Tooltip(tip)
	b.Background(t.Surface)
	b.Border(1, t.Border)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 13)).Size(13, 13).TextColor(t.Accent)
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(600)
	})
	return b
}

func (a *app) ghostButton(c *ui.Context, label, ic string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(32).Padding(0, 9).Radius(7).Gap(6).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 14)).Size(14, 14).TextColor(t.TextMuted)
		}
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
	})
	return b
}

func (a *app) navItem(c *ui.Context, page, label, ic string, count string) ui.Element {
	t := c.Theme()
	p := a.palette
	on := a.page == page
	b := ui.ButtonBase(c).Key("nav:" + page).FillWidth().Height(38).PaddingX(10).Radius(9).Gap(10).FocusRing(false).Label(label)
	if on {
		b.Background(p.Accent.Alpha(0.12))
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, icon(ic, 16)).Size(16, 16).TextColor(func() ui.Color {
			if on {
				return p.Accent
			}
			return p.Muted
		}())
		ui.Text(c, label).FontSize(12.8).FontWeight(560).Grow(1).SingleLine().TextColor(func() ui.Color {
			if on {
				return p.Text
			}
			return p.Text
		}())
		if count != "" {
			ui.Text(c, count).FontSize(10.5).TextColor(p.Muted).Padding(2, 6).Radius(99).Background(p.Field)
		}
	})
	if b.Clicked() {
		a.page = page
	}
	return b
}

func methodColor(p palette, method string) ui.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return p.Good
	case "POST":
		return p.Warn
	case "PUT":
		return ui.Hex("#69a7ff")
	case "PATCH":
		return ui.Hex("#b18cff")
	case "DELETE":
		return p.Bad
	case "HEAD", "OPTIONS":
		return p.Muted
	default:
		return p.Muted
	}
}

func (a *app) methodChip(c *ui.Context, method string, width float32) ui.Element {
	p := a.palette
	col := methodColor(p, method)
	return ui.Row(c).Height(24).Width(width).Center().Radius(6).Background(col.Alpha(0.13)).Children(func() {
		ui.Text(c, strings.ToUpper(method)).FontSize(10).FontWeight(700).LetterSpacing(0.2).TextColor(col)
	})
}

func (a *app) dot(c *ui.Context, color ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Radius(size).Background(color)
}

func (a *app) kvHeader(c *ui.Context) {
	p := a.palette
	ui.Row(c).FillWidth().Height(24).PaddingX(4).Children(func() {
		ui.Box(c).Width(26)
		ui.Text(c, "Key").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Grow(1)
		ui.Text(c, "Value").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Grow(1.35)
		ui.Box(c).Width(28)
	})
}

func (a *app) paramHeader(c *ui.Context) {
	p := a.palette
	ui.Row(c).FillWidth().Height(24).PaddingX(4).Children(func() {
		ui.Box(c).Width(26)
		ui.Text(c, "Key").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Grow(1)
		ui.Text(c, "Value").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Grow(1.35)
		ui.Box(c).Width(28)
	})
}

func (a *app) paramRows(c *ui.Context, r *core.Request) {
	for i := range r.Query {
		row := &r.Query[i]
		idx := i
		ui.Row(c).FillWidth().Gap(7).Padding(3, 2).Children(func() {
			cb := ui.Checkbox(c, &row.Enabled, "").Width(22).Label("Enabled")
			if cb.Changed() {
				core.SyncURLQuery(r)
				a.save()
			}
			key := a.textInput(c, &row.Key).Placeholder("Parameter").Height(32).Grow(1).FontSize(12).Label("Parameter key")
			key = a.variableField(c, key, &row.Key, "", 12, "param-key:"+row.ID, nil, true)
			val := a.textInput(c, &row.Value).Placeholder("Value").Height(32).Grow(1.35).FontSize(12).Label("Parameter value")
			val = a.variableField(c, val, &row.Value, "", 12, "param-val:"+row.ID, nil, true)
			if key.Changed() || val.Changed() {
				core.SyncURLQuery(r)
				a.save()
			}
			x := a.iconButton(c, "x", "Remove", 28).Key(fmt.Sprintf("param-del-%d", idx))
			if x.Clicked() {
				r.Query = append(r.Query[:idx], r.Query[idx+1:]...)
				core.SyncURLQuery(r)
				a.save()
			}
		}).Key(fmt.Sprintf("param-%d", idx))
	}
}

func (a *app) kvRows(c *ui.Context, rowsPtr *[]core.KV, idPrefix string, onChange func()) {
	rows := *rowsPtr
	for i := range rows {
		row := &rows[i]
		idx := i
		ui.Row(c).FillWidth().Gap(7).Padding(3, 2).Children(func() {
			cb := ui.Checkbox(c, &row.Enabled, "").Width(22).Label("Enabled")
			if cb.Changed() {
				onChange()
			}
			key := a.textInput(c, &row.Key).Placeholder("Key").Height(32).Grow(1).FontSize(12).
				Label("Header key").OnTextKey(a.headerTextKey(c, row, "key"))
			key = a.variableField(c, key, &row.Key, "", 12, idPrefix+"key:"+row.ID, a.headerTextKey(c, row, "key"), true)
			var val ui.Element
			if row.Secret {
				val = a.secretInput(c, &row.Value)
			} else {
				val = a.textInput(c, &row.Value)
			}
			val = val.Placeholder("Value").Height(32).Grow(1.35).FontSize(12).
				Label("Value").OnTextKey(a.headerTextKey(c, row, "value"))
			val = a.variableField(c, val, &row.Value, "", 12, idPrefix+"val:"+row.ID, a.headerTextKey(c, row, "value"), true)
			if val.Changed() || key.Changed() {
				onChange()
			}
			x := a.iconButton(c, "x", "Remove", 28).Key(fmt.Sprintf("%s-del-%d", idPrefix, idx))
			if x.Clicked() {
				*rowsPtr = append(rows[:idx], rows[idx+1:]...)
				if a.headerSuggest.row == row.ID {
					a.headerSuggest = headerSuggestState{}
				}
				if a.headerFocus == row.ID {
					a.headerFocus = ""
				}
				if onChange != nil {
					onChange()
				}
			}

			// Header autocomplete. The popover opens under the field being
			// typed in; both entrances are driven every frame so a closed
			// popover sits at the start of its animation for the next open.
			keyFocused := key.Focused()
			valFocused := val.Focused()
			var items []core.Suggestion
			field := ""
			switch {
			case keyFocused:
				field = "key"
				items = a.headerFieldSuggestions(row, "key", row.Key)
			case valFocused:
				field = "value"
				items = a.headerFieldSuggestions(row, "value", row.Value)
			default:
				if a.headerSuggest.row == row.ID {
					a.headerSuggest = headerSuggestState{}
				}
			}
			keyTarget := float32(0)
			if keyFocused && a.headerSuggest.open {
				keyTarget = 1
			}
			valTarget := float32(0)
			if valFocused && a.headerSuggest.open {
				valTarget = 1
			}
			keyAnim := key.Animate(headerSuggestAnimKey(row, "key"), keyTarget, headerSuggestDuration)
			valAnim := val.Animate(headerSuggestAnimKey(row, "value"), valTarget, headerSuggestDuration)
			switch field {
			case "key":
				a.headerSuggestPopover(c, key, row, "key", items, keyAnim)
			case "value":
				a.headerSuggestPopover(c, val, row, "value", items, valAnim)
			}
			if a.headerFocus == row.ID {
				a.headerFocus = ""
				a.inputHandle(val.ID()).Focus(c)
			}
		}).Key(fmt.Sprintf("%s-%d", idPrefix, idx))
	}
}

func (a *app) addRow(c *ui.Context, label string) bool {
	b := a.ghostButton(c, label, "plus")
	return b.Clicked()
}

func (a *app) tabButton(c *ui.Context, label string, selected bool, count string) ui.Element {
	t := c.Theme()
	p := a.palette
	b := ui.ButtonBase(c).Height(30).PaddingX(11).Radius(7).Gap(6).Label(label).
		Transition(ui.ElementTransition{Duration: 140 * time.Millisecond, Colors: true})
	if selected {
		b.Background(p.Field)
		b.Border(1, p.Border)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(func() int {
			if selected {
				return 650
			}
			return 500
		}()).TextColor(func() ui.Color {
			if selected {
				return p.Text
			}
			return p.Muted
		}())
		if count != "" {
			ui.Text(c, count).FontSize(10).TextColor(p.Muted)
		}
	})
	return b
}

func (a *app) statusPill(c *ui.Context, code int) ui.Element {
	p := a.palette
	col := p.Muted
	if code > 0 {
		switch core.StatusKind(code) {
		case "ok":
			col = p.Good
		case "redirect":
			col = ui.Hex("#69a7ff")
		case "warn":
			col = p.Warn
		case "error":
			col = p.Bad
		}
	}
	return ui.Row(c).Height(24).PaddingX(9).Gap(6).Radius(99).Background(col.Alpha(0.12)).Center().Children(func() {
		a.dot(c, col, 5)
		ui.Text(c, fmt.Sprintf("%d", code)).FontSize(11).FontWeight(700).TextColor(col)
	})
}

func (a *app) emptyState(c *ui.Context, ic, title, body string, action string) bool {
	p := a.palette
	clicked := false
	ui.Column(c).Fill().Center().Gap(10).Padding(28).Children(func() {
		ui.Box(c).Size(52, 52).Radius(16).Background(p.Accent.Alpha(0.10)).Center().Children(func() {
			ui.Icon(c, icon(ic, 26)).Size(24, 24).TextColor(p.Accent)
		})
		ui.Text(c, title).FontSize(15).FontWeight(650).TextAlign(ui.Center)
		ui.Text(c, body).FontSize(12).TextColor(p.Muted).TextAlign(ui.Center).MaxWidth(330).Wrap()
		if action != "" {
			clicked = a.accentButton(c, action, "plus").Clicked()
		}
	})
	return clicked
}

func (a *app) hdivider(c *ui.Context) ui.Element {
	p := a.palette
	return ui.Box(c).FillWidth().Height(1).Background(p.Border)
}

func (a *app) listRow(c *ui.Context, selected bool, fn func()) ui.Element {
	t := c.Theme()
	p := a.palette
	row := ui.Row(c).FillWidth().MinHeight(38).Padding(0, 10).Radius(8).AlignItems(ui.Center).Gap(9)
	if selected {
		row.Background(p.Accent.Alpha(0.10))
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(fn)
	return row
}

func (a *app) badge(c *ui.Context, text string, color ui.Color) ui.Element {
	return ui.Text(c, text).FontSize(10).FontWeight(650).Padding(3, 7).Radius(99).Background(color.Alpha(0.12)).TextColor(color)
}

func (a *app) keyValueHint(c *ui.Context, text string) {
	p := a.palette
	ui.Text(c, text).FontSize(11).TextColor(p.Muted).Wrap()
}
