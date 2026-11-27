package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *app) textInput(c *ui.Context, value *string) ui.Element {
	return a.inputMenu(c, ui.TextInput(c, value), value, false, false)
}

func (a *app) secretInput(c *ui.Context, value *string) ui.Element {
	return a.inputMenu(c, ui.TextInput(c, value).Password(), value, true, false)
}

func (a *app) textArea(c *ui.Context, value *string) ui.Element {
	return a.inputMenu(c, ui.TextArea(c, value), value, false, true)
}

// inputMenu replaces MyGo's native text-field menu with the app's themed one.
// The build function runs when the secondary button opens the menu; opening the
// themed menu there keeps MyGo from showing its native edit menu.
func (a *app) inputMenu(c *ui.Context, el ui.Element, value *string, secret, multiline bool) ui.Element {
	h := a.inputHandle(el.ID())
	token := a.viewToken
	el = el.Bind(h)
	return el.ContextMenu(func(m *ui.Menu) {
		a.openContextMenuAtToken(token, el, a.inputEditMenu(c, h, el, value, secret, multiline))
	})
}

func (a *app) inputHandle(id uint64) *ui.Handle {
	if a.inputHandles == nil {
		a.inputHandles = map[uint64]*ui.Handle{}
	}
	h := a.inputHandles[id]
	if h == nil {
		h = &ui.Handle{}
		a.inputHandles[id] = h
	}
	return h
}

func (a *app) inputEditMenu(c *ui.Context, h *ui.Handle, el ui.Element, value *string, secret, multiline bool) []contextMenuItem {
	runes := []rune(*value)
	start, end := el.TextSelection()
	if start > end {
		start, end = end, start
	}
	start = max(0, min(start, len(runes)))
	end = max(0, min(end, len(runes)))
	hasSelection := end > start
	selected := string(runes[start:end])

	cut := menuAction("Cut", "scissors", func() {
		c.WriteClipboard(selected)
		replaceInputValue(c, h, value, runes, start, end, nil)
	})
	cut.Disabled = secret || !hasSelection

	copyItem := menuAction("Copy", "copy", func() {
		c.WriteClipboard(selected)
	})
	copyItem.Disabled = secret || !hasSelection

	paste := menuAction("Paste", "clipboard", func() {
		text := c.ReadClipboard()
		if !multiline {
			text = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text)
		}
		if text == "" {
			return
		}
		replaceInputValue(c, h, value, runes, start, end, []rune(text))
	})

	del := menuAction("Delete", "trash", func() {
		replaceInputValue(c, h, value, runes, start, end, nil)
	})
	del.Disabled = !hasSelection

	selectAll := menuAction("Select all", "select-all", func() {
		c.Resolve(*h).SetTextSelection(0, len(runes))
	})

	return []contextMenuItem{
		cut,
		copyItem,
		paste,
		del,
		menuSeparator(),
		selectAll,
	}
}

// selectableMenu replaces MyGo's native menu on read-only selectable text,
// such as the response body, with the app's themed one. Copy uses the current
// selection; Copy all gives the same result as selecting the whole paragraph.
func (a *app) selectableMenu(c *ui.Context, el ui.Element, text string) ui.Element {
	token := a.viewToken
	return el.ContextMenu(func(m *ui.Menu) {
		var selected strings.Builder
		for i, part := range el.SelectedTexts() {
			r := []rune(part.Text)
			start, end := max(0, min(part.Start, len(r))), max(0, min(part.End, len(r)))
			if start > end {
				start, end = end, start
			}
			if i > 0 {
				selected.WriteString("\n")
			}
			selected.WriteString(string(r[start:end]))
		}
		sel := selected.String()
		copyItem := menuAction("Copy", "copy", func() { c.WriteClipboard(sel) })
		copyItem.Disabled = sel == ""
		a.openContextMenuAtToken(token, el, []contextMenuItem{
			copyItem,
			menuAction("Copy all", "clipboard", func() { c.WriteClipboard(text) }),
		})
	})
}

func replaceInputValue(c *ui.Context, h *ui.Handle, value *string, runes []rune, start, end int, insert []rune) {
	next := make([]rune, 0, len(runes)-(end-start)+len(insert))
	next = append(next, runes[:start]...)
	next = append(next, insert...)
	next = append(next, runes[end:]...)
	*value = string(next)
	caret := start + len(insert)
	c.Resolve(*h).SetTextSelection(caret, caret)
}
