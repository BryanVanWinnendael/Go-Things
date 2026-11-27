package app

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const (
	maxAttachmentSize  = int64(25 << 20)
	maxAttachmentTotal = int64(25 << 20)
)

// composeAttachment is a file chosen in the compose view. Its bytes are
// read only when the message is sent.
type composeAttachment struct {
	Path string
	Name string
	Size int64
}

func (a *App) pickAttachments() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      a.mainWindow,
		Title:       "Attach files",
		ButtonLabel: "Attach",
		Multiple:    true,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	a.addAttachmentPaths(paths)
	a.invalidate()
}

func (a *App) addAttachmentPaths(paths []string) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		name := filepath.Base(path)
		if info.Size() > maxAttachmentSize {
			a.composeErr = fmt.Sprintf("%s is too large. The limit is %s per file.", name, formatBytes(maxAttachmentSize))
			continue
		}
		var total int64
		for _, att := range a.composeAttachments {
			total += att.Size
		}
		if total+info.Size() > maxAttachmentTotal {
			a.composeErr = fmt.Sprintf("Attachments are too large. The limit is %s in total.", formatBytes(maxAttachmentTotal))
			break
		}
		a.composeAttachments = append(a.composeAttachments, composeAttachment{Path: path, Name: name, Size: info.Size()})
	}
}

func readOutgoingAttachments(atts []composeAttachment) ([]OutgoingAttachment, error) {
	out := make([]OutgoingAttachment, 0, len(atts))
	var total int64
	for _, att := range atts {
		data, err := os.ReadFile(att.Path)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", att.Name, err)
		}
		if int64(len(data)) > maxAttachmentSize {
			return nil, fmt.Errorf("%s is too large. The limit is %s per file.", att.Name, formatBytes(maxAttachmentSize))
		}
		total += int64(len(data))
		if total > maxAttachmentTotal {
			return nil, fmt.Errorf("Attachments are too large. The limit is %s in total.", formatBytes(maxAttachmentTotal))
		}
		out = append(out, OutgoingAttachment{
			Name: att.Name,
			Type: mime.TypeByExtension(filepath.Ext(att.Name)),
			Data: data,
		})
	}
	return out, nil
}

// signatureText is the active account's signature, trimmed for insertion.
// It is plain text and is placed above quoted text in replies and forwards.
func (a *App) signatureText() string {
	return strings.TrimSpace(a.acct.Signature)
}

// composeEditorDoc returns the rich document the compose view edits, building
// it from the account's signature the first time.
func (a *App) composeEditorDoc() *richtext.Document {
	if a.composeDoc == nil {
		a.startComposeDoc()
	}
	return a.composeDoc
}

// accountSignatureText is the signature as rich text, with its images loaded,
// so it can be placed straight into the compose document.
func (a *App) accountSignatureText() richtext.Text {
	ed := a.signatureDoc()
	return ed.doc.Text()
}

// startComposeDoc builds a fresh compose document: the typed body, the
// account's signature below it, then the quoted message of a reply or
// forward. The caret starts above the signature so typing goes first.
func (a *App) startComposeDoc() {
	text := richtext.NewText("")
	has := false
	add := func(t richtext.Text) {
		if has {
			text = text.Insert(text.Len(), richtext.NewText("\n\n"))
		}
		text = text.Insert(text.Len(), t)
		has = true
	}
	if body := strings.TrimRight(a.composeBody, "\n"); body != "" {
		add(richtext.NewText(body))
	} else {
		// Start on an empty line so typing goes above the signature.
		add(richtext.NewText(""))
	}
	if sig := a.accountSignatureText(); strings.TrimSpace(sig.PlainText()) != "" {
		add(sig)
	}
	if q := strings.TrimSpace(a.composeQuote); q != "" {
		add(richtext.NewText(strings.TrimLeft(q, "\n")))
	}
	doc := richtext.NewDocument(text)
	doc.SetSelection(0, 0)
	a.composeDoc = doc
	a.composeColor = a.palette.Text
	if a.composeColor.A == 0 {
		a.composeColor = ui.Hex("#111111")
	}
}

// composeToolbar draws the formatting controls of the compose editor.
func (a *App) composeToolbar(c *ui.Context) {
	p := a.palette
	doc := a.composeEditorDoc()
	f := doc.Formatting()
	ui.Row(c).Gap(5).Wrap().AlignItems(ui.Center).Children(func() {
		font := "System"
		for _, name := range signatureFonts {
			if signatureFontValues[name] == f.Attributes.Font {
				font = name
				break
			}
		}
		if ui.Select(c, &font, signatureFonts).Width(142).KeepFocus().Label("Font").Changed() {
			family := signatureFontValues[font]
			doc.Format(func(at *richtext.Attributes) { at.Font = family })
		}

		size := "14"
		if f.Attributes.Size > 0 {
			size = strconv.Itoa(int(f.Attributes.Size + 0.5))
		}
		if ui.Select(c, &size, signatureSizes).Width(62).KeepFocus().Label("Size").Changed() {
			if px, err := strconv.ParseFloat(size, 32); err == nil {
				doc.Format(func(at *richtext.Attributes) { at.Size = float32(px) })
			}
		}

		a.composeToolButton(c, "B", "Bold", f.Bold == richtext.FormatOn, doc.ToggleBold)
		a.composeToolButton(c, "I", "Italic", f.Italic == richtext.FormatOn, doc.ToggleItalic)
		a.composeToolButton(c, "U", "Underline", f.Underline == richtext.FormatOn, doc.ToggleUnderline)
		a.composeToolButton(c, "S", "Strikethrough", f.Strikethrough == richtext.FormatOn, doc.ToggleStrikethrough)

		ui.Box(c).Width(1).Height(20).Background(p.Border)

		if f.Attributes.Color.A > 0 {
			a.composeColor = f.Attributes.Color
		}
		well := richColorWell(c, &a.composeColor, "Text color")
		if richColorWellChanged(well) {
			doc.Format(func(at *richtext.Attributes) { at.Color = a.composeColor })
		}

		ui.Box(c).Width(1).Height(20).Background(p.Border)

		for _, align := range []struct {
			label string
			value ui.Align
		}{
			{"Left", ui.Start},
			{"Center", ui.Center},
			{"Right", ui.End},
		} {
			align := align
			on := !f.ParagraphsMixed && f.Paragraph.Align == align.value
			a.composeToolButton(c, align.label, "Align "+align.label, on, func() {
				doc.SetAlign(align.value)
			})
		}

		ui.Box(c).Width(1).Height(20).Background(p.Border)

		a.composeToolButton(c, "Undo", "Undo", false, func() {
			if doc.CanUndo() {
				doc.Undo()
			}
		})
		a.composeToolButton(c, "Redo", "Redo", false, func() {
			if doc.CanRedo() {
				doc.Redo()
			}
		})
	})
}

// composeToolButton draws one compose formatting control.
func (a *App) composeToolButton(c *ui.Context, label, tip string, on bool, run func()) ui.Element {
	p := a.palette
	b := ui.Button(c, label).KeepFocus().Padding(4, 8).MinWidth(28).Radius(6).Tooltip(tip)
	if on {
		b.Background(p.Accent.Alpha(0.18))
		b.TextColor(p.Accent)
	}
	if b.Clicked() {
		run()
	}
	return b
}

// composeMessageParts returns what send needs from the editor: the plain
// text, the HTML and the inline image bytes. An empty HTML means the caller
// should fall back to the plain BuildMessage path.
func (a *App) composeMessageParts() (plain, html string, data [][]byte, types []string) {
	doc := a.composeDoc
	if doc == nil {
		body := a.composeBody
		if sig := a.signatureText(); sig != "" {
			body = strings.TrimRight(body, "\n") + "\n\n" + sig
		}
		if q := strings.TrimSpace(a.composeQuote); q != "" {
			body = strings.TrimRight(body, "\n") + "\n" + a.composeQuote
		}
		return body, "", nil, nil
	}
	text := renumberSignatureAttachments(doc.Text(), &[]string{})
	plain = strings.ReplaceAll(text.PlainText(), string(richtext.ObjectReplacement), "")
	html = strings.TrimPrefix(text.HTML(), `<meta charset="utf-8">`)
	for _, p := range text.Paragraphs() {
		for _, r := range p.Runs {
			att := r.Attributes.Attachment
			if att == nil {
				continue
			}
			raw, _ := att.Data.(string)
			d, ct := decodeSignatureImage(raw)
			if len(d) == 0 {
				// Keep the part slot so later cid: indices still line up.
				data = append(data, nil)
				types = append(types, "application/octet-stream")
				continue
			}
			data = append(data, d)
			types = append(types, ct)
		}
	}
	return plain, html, data, types
}
