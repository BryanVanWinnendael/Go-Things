package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const (
	signatureCanvasW = float32(600)
	signatureCanvasH = float32(260)
)

// richColorWell is a text-color swatch like ui.ColorWell, but its picker is
// anchored to the swatch's right edge so the panel opens to the left. MyGo's
// own ColorWell attaches to the start, which makes the picker stick out past
// the right edge of the signature and compose toolbars.
func richColorWell(c *ui.Context, color *ui.Color, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Width(38).Shrink(0).Padding(3).Radius(t.Radius).
		Background(t.Surface).Border(1, t.Border).Label(label).KeepFocus()
	open := ui.Local(b, "richColorWellOpen", func() bool { return false })
	changed := ui.Local(b, "richColorWellChanged", func() bool { return false })
	if b.Clicked() {
		*open = !*open
	}
	now := *color
	b.Children(func() {
		ui.Box(c).FillWidth().Height(14).Radius(2).Clip().Shrink(0).
			Draw(func(pt *ui.Painter, r ui.Rect) { pt.Fill(r, now, 0) })
	})
	// Compare around the picker build so only a choice made in the picker is
	// reported as a change, not the editor updating the swatch from its format.
	before := *color
	ui.PopoverBase(c, b, open, func(panel ui.Element) {
		panel.Label("Color picker").AttachTo(b, ui.AnchorBottomEnd, ui.AnchorTopEnd)
		ui.ColorPicker(c, color)
	})
	*changed = *color != before
	return b
}

// richColorWellChanged reports whether the picker changed the color this frame.
func richColorWellChanged(e ui.Element) bool {
	v := ui.Local(e, "richColorWellChanged", func() bool { return false })
	return v != nil && *v
}

// signatureFonts are the families a signature can use. The empty value
// takes the system font, as the rest of the app does.
var signatureFonts = []string{
	"System", "Arial", "Georgia", "Times New Roman",
	"Courier New", "Verdana", "Trebuchet MS", "Tahoma",
}

var signatureFontValues = map[string]string{
	"System":          "",
	"Arial":           "Arial, Helvetica, sans-serif",
	"Georgia":         "Georgia, serif",
	"Times New Roman": "Times New Roman, Times, serif",
	"Courier New":     "Courier New, Courier, monospace",
	"Verdana":         "Verdana, Geneva, sans-serif",
	"Trebuchet MS":    "Trebuchet MS, sans-serif",
	"Tahoma":          "Tahoma, sans-serif",
}

var signatureSizes = []string{"10", "11", "12", "13", "14", "16", "18", "20", "24", "28", "32"}

// signatureEditor keeps the live rich-text document of the account whose
// signature the settings window shows, and the decoded images of its canvas.
type signatureEditor struct {
	doc     *richtext.Document
	email   string
	version uint64
	sel     int
	color   ui.Color
	bmps    map[string]*ui.Bitmap
}

// signatureDoc returns the editor of the active account's signature,
// loading it from the account the first time that account is shown.
func (a *App) signatureDoc() *signatureEditor {
	if a.sigEd != nil && a.sigEd.email == a.acct.Email {
		return a.sigEd
	}
	text := richtext.NewText(a.acct.Signature)
	if rich := strings.TrimSpace(a.acct.SignatureRich); rich != "" {
		var saved richtext.Text
		if err := json.Unmarshal([]byte(rich), &saved); err == nil {
			// The rich text codec does not store attachment bytes, so match
			// them back up from the account. Signatures saved by the old
			// canvas editor have no SignatureImages yet; fall back to the
			// canvas bytes so their image is not lost, and persist them so
			// the image survives even if the canvas is later cleared.
			imgs := a.acct.SignatureImages
			fromCanvas := false
			if len(imgs) == 0 {
				for _, im := range a.acct.SignatureCanvas {
					imgs = append(imgs, im.Data)
				}
				fromCanvas = len(imgs) > 0
			}
			saved = attachPersistedImages(saved, imgs)
			saved = renumberSignatureAttachments(saved, &[]string{})
			text = loadSignatureAttachments(saved)
			if fromCanvas {
				if data := signatureImageData(text); len(data) > 0 {
					a.acct.SignatureImages = data
					if a.active >= 0 && a.active < len(a.accounts) {
						a.accounts[a.active] = a.acct
					}
					a.saveConfig()
				}
			}
		}
	}
	// Legacy canvas images: append them inline so the new editor can
	// manage them. (One-time, in-memory; saveSignature persists.)
	if len(a.acct.SignatureCanvas) > 0 && !hasInlineImages(text) {
		for _, im := range a.acct.SignatureCanvas {
			raw, _ := decodeSignatureImage(im.Data)
			if len(raw) == 0 {
				continue
			}
			bmp, err := ui.DecodeBitmap(raw)
			if err != nil {
				continue
			}
			w, h := bmp.Size()
			width := float32(w)
			height := float32(h)
			if width > 240 {
				scale := 240.0 / float64(width)
				width = 240
				height = float32(float64(height) * scale)
			}
			if width < 1 {
				width, height = 1, 1
			}
			ct := http.DetectContentType(raw)
			if !strings.HasPrefix(ct, "image/") {
				continue
			}
			att := &richtext.Attachment{
				Image:  bmp,
				Width:  width,
				Height: height,
				Name:   "",
				Data:   base64.StdEncoding.EncodeToString(raw),
			}
			imgText := richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: att}}}})
			text = text.Insert(text.Len(), richtext.NewText("\n"))
			text = text.Insert(text.Len(), imgText)
		}
	}
	doc := richtext.NewDocument(text)
	a.sigEd = &signatureEditor{
		doc:     doc,
		email:   a.acct.Email,
		version: doc.Version(),
		sel:     -1,
		color:   ui.Hex("#111111"),
		bmps:    map[string]*ui.Bitmap{},
	}
	return a.sigEd
}

func hasInlineImages(t richtext.Text) bool {
	for _, p := range t.Paragraphs() {
		for _, r := range p.Runs {
			if r.Attributes.Attachment != nil {
				return true
			}
		}
	}
	return false
}

func renumberSignatureAttachments(text richtext.Text, imgs *[]string) richtext.Text {
	var paras []richtext.Paragraph
	idx := 0
	for _, p := range text.Paragraphs() {
		para := richtext.Paragraph{Style: p.Style}
		for _, r := range p.Runs {
			run := r
			if r.Attributes.Attachment != nil {
				att := *r.Attributes.Attachment
				att.URL = fmt.Sprintf("cid:signature-image-%d", idx)
				if data, ok := att.Data.(string); ok && data != "" {
					*imgs = append(*imgs, data)
				} else {
					*imgs = append(*imgs, "")
				}
				idx++
				run.Attributes.Attachment = &att
			}
			para.Runs = append(para.Runs, run)
		}
		paras = append(paras, para)
	}
	return richtext.TextOf(paras...)
}

// attachPersistedImages fills each attachment's Data with the persisted
// base64 bytes, matched by document order.
func attachPersistedImages(text richtext.Text, imgs []string) richtext.Text {
	var paras []richtext.Paragraph
	i := 0
	for _, p := range text.Paragraphs() {
		para := richtext.Paragraph{Style: p.Style}
		for _, r := range p.Runs {
			run := r
			if r.Attributes.Attachment != nil {
				att := *r.Attributes.Attachment
				if i < len(imgs) && att.Data == nil {
					att.Data = imgs[i]
				}
				i++
				run.Attributes.Attachment = &att
			}
			para.Runs = append(para.Runs, run)
		}
		paras = append(paras, para)
	}
	return richtext.TextOf(paras...)
}

// signatureImageData returns the base64 bytes of each inline attachment, in
// document order.
func signatureImageData(text richtext.Text) []string {
	var data []string
	for _, p := range text.Paragraphs() {
		for _, r := range p.Runs {
			if r.Attributes.Attachment == nil {
				continue
			}
			if s, ok := r.Attributes.Attachment.Data.(string); ok {
				data = append(data, s)
			} else {
				data = append(data, "")
			}
		}
	}
	return data
}

func loadSignatureAttachments(text richtext.Text) richtext.Text {
	var paras []richtext.Paragraph
	for _, p := range text.Paragraphs() {
		para := richtext.Paragraph{Style: p.Style}
		for _, r := range p.Runs {
			run := r
			if r.Attributes.Attachment != nil {
				att := *r.Attributes.Attachment
				if att.Image == nil {
					if data, ok := att.Data.(string); ok {
						raw, err := base64.StdEncoding.DecodeString(data)
						if err == nil {
							if bmp, err := ui.DecodeBitmap(raw); err == nil {
								att.Image = bmp
								if att.Width <= 0 || att.Height <= 0 {
									w, h := bmp.Size()
									att.Width = float32(w)
									att.Height = float32(h)
								}
								if att.Width > 400 {
									scale := 400.0 / float64(att.Width)
									att.Width = 400
									att.Height = float32(float64(att.Height) * scale)
								}
								if att.Width < 1 {
									att.Width, att.Height = 1, 1
								}
							}
						}
					}
				}
				run.Attributes.Attachment = &att
			}
			para.Runs = append(para.Runs, run)
		}
		paras = append(paras, para)
	}
	return richtext.TextOf(paras...)
}

// insertSignatureImage opens a file picker and inserts each chosen image
// into the signature text at the caret, like a normal editor.
func (a *App) insertSignatureImage() {
	parent := a.mainWindow
	if a.settingsWin != nil {
		parent = a.settingsWin
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      parent,
		Title:       "Insert image",
		ButtonLabel: "Insert",
		Multiple:    true,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	ed := a.signatureDoc()
	doc := ed.doc
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) > 500*1024 {
			continue
		}
		ct := http.DetectContentType(data)
		if !strings.HasPrefix(ct, "image/") {
			continue
		}
		bmp, err := ui.DecodeBitmap(data)
		if err != nil {
			continue
		}
		w, h := bmp.Size()
		width := float32(w)
		height := float32(h)
		if width > 80 {
			scale := 80.0 / float64(width)
			width = 80
			height = float32(float64(height) * scale)
		}
		if width < 1 {
			width, height = 1, 1
		}
		att := &richtext.Attachment{
			Image:  bmp,
			Width:  width,
			Height: height,
			Name:   "",
			Data:   base64.StdEncoding.EncodeToString(data),
		}
		imgText := richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: att}}}})
		anchor, caret := doc.Selection()
		pos := caret
		if anchor != caret {
			if anchor < caret {
				doc.Edit(func(e *richtext.Edit) { e.Replace(anchor, caret, imgText) })
				pos = anchor + 1
			} else {
				doc.Edit(func(e *richtext.Edit) { e.Replace(caret, anchor, imgText) })
				pos = caret + 1
			}
		} else {
			// Put the image on its own line so it does not overlap text.
			prefixed := richtext.TextOf(
				richtext.Paragraph{Runs: []richtext.Run{{Text: "\n"}}},
				richtext.Paragraph{Runs: []richtext.Run{{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: att}}}},
				richtext.Paragraph{Runs: []richtext.Run{{Text: "\n"}}},
			)
			doc.Edit(func(e *richtext.Edit) { e.Replace(pos, pos, prefixed) })
			pos += 3
		}
		doc.SetSelection(pos, pos)
	}
	a.saveSignature()
	a.invalidate()
}

// saveSignature stores the rich signature and the plain text compose
// inserts while the message is written.
func (a *App) saveSignature() {
	ed := a.sigEd
	if ed == nil {
		return
	}
	text := ed.doc.Text()
	// Collect inline image data in document order, and renumber CIDs.
	var imgs []string
	renumbered := renumberSignatureAttachments(text, &imgs)
	if data, err := json.Marshal(renumbered); err == nil {
		a.acct.SignatureRich = string(data)
	}
	// Persist image bytes separately (the codec does not keep them).
	a.acct.SignatureImages = imgs
	a.acct.SignatureHTML = ""
	a.acct.Signature = renumbered.PlainText()
	ed.version = ed.doc.Version()
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active] = a.acct
	}
	a.saveConfig()
}
func (a *App) alignSignatureImage(idx int, x float32) {
	if idx < 0 || idx >= len(a.acct.SignatureCanvas) {
		return
	}
	im := &a.acct.SignatureCanvas[idx]
	im.X = clampSignature(x, 0, 100-im.Width)
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active].SignatureCanvas = a.acct.SignatureCanvas
	}
	a.saveSignatureCanvas()
	a.invalidate()
}

func (a *App) saveSignatureCanvas() {
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active] = a.acct
	}
	a.saveConfig()
}

// signatureBitmap decodes and caches the picture of a canvas image.
func (ed *signatureEditor) signatureBitmap(data string) *ui.Bitmap {
	if bmp, ok := ed.bmps[data]; ok {
		return bmp
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil
	}
	bmp, err := ui.DecodeBitmap(raw)
	if err != nil {
		return nil
	}
	ed.bmps[data] = bmp
	return bmp
}

func clampSignature(v, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// richSignatureHTML returns the signature's rich text, without the meta
// tag Text.HTML adds, falling back to HTML or plain text saved by older
// versions.
func richSignatureHTML(a Account) string {
	if rich := strings.TrimSpace(a.SignatureRich); rich != "" {
		var text richtext.Text
		if err := json.Unmarshal([]byte(rich), &text); err == nil {
			text = renumberSignatureAttachments(text, &[]string{})
			return strings.TrimPrefix(text.HTML(), `<meta charset="utf-8">`)
		}
	}
	if legacy := strings.TrimSpace(a.SignatureHTML); legacy != "" {
		return strings.TrimPrefix(legacy, `<meta charset="utf-8">`)
	}
	if strings.TrimSpace(a.Signature) == "" {
		return ""
	}
	return textToHTML(strings.TrimSpace(a.Signature))
}

// signatureHTML renders the rich signature. Images are inline in the rich
// text now; legacy canvas images are appended for old signatures.
func signatureHTML(a Account) string {
	rich := richSignatureHTML(a)
	if !strings.Contains(rich, "<img") && len(a.SignatureCanvas) > 0 {
		var imgs strings.Builder
		idx := inlineImageCount(a)
		for _, im := range a.SignatureCanvas {
			raw, _ := decodeSignatureImage(im.Data)
			if len(raw) == 0 {
				continue
			}
			wpx := int(im.Width / 100 * 600)
			if wpx < 12 {
				wpx = 120
			}
			fmt.Fprintf(&imgs, `<br><img src="cid:signature-image-%d" width="%d" alt="" style="display:block;border:0;outline:none;height:auto;max-width:100%%;">`, idx, wpx)
			idx++
		}
		rich += imgs.String()
	}
	if strings.TrimSpace(rich) == "" {
		return ""
	}
	// A light card keeps the signature readable in every mail client,
	// whatever the app theme is.
	return `<div style="background:#ffffff;color:#111111;padding:8px 10px;border-radius:4px;margin-top:8px;">` + rich + `</div>`
}

func inlineImageCount(a Account) int {
	n := 0
	if rich := strings.TrimSpace(a.SignatureRich); rich != "" {
		var text richtext.Text
		if err := json.Unmarshal([]byte(rich), &text); err == nil {
			for _, p := range text.Paragraphs() {
				for _, r := range p.Runs {
					if r.Attributes.Attachment != nil {
						n++
					}
				}
			}
		}
	}
	return n
}

func legacySignatureImages(a Account) []string {
	if len(a.SignatureImages) > 0 {
		return a.SignatureImages
	}
	if a.SignatureImage != "" {
		return []string{a.SignatureImage}
	}
	return nil
}

// composeHTML turns the composed message into its HTML part, replacing the
// plain signature the body carries with the rich one. It reports whether
// the rich signature was used, which needs a multipart message even
// without images or files.
func composeHTML(a Account, body string) (string, bool) {
	rich := signatureHTML(a)
	sig := strings.TrimSpace(a.Signature)
	if rich != "" && sig != "" {
		if i := strings.Index(body, sig); i >= 0 {
			return textToHTML(body[:i]) + rich + textToHTML(body[i+len(sig):]), true
		}
	}
	if rich != "" && sig == "" {
		return textToHTML(body) + "<br>" + rich, true
	}
	return textToHTML(body), false
}

// signatureEditorView draws the rich signature editor: a formatting bar, a
// page the user types on, and the images placed on that page.
func (a *App) signatureEditorView(c *ui.Context) {
	p := a.palette
	ed := a.signatureDoc()
	doc := ed.doc
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
			a.saveSignature()
		}

		size := "14"
		if f.Attributes.Size > 0 {
			size = strconv.Itoa(int(f.Attributes.Size + 0.5))
		}
		if ui.Select(c, &size, signatureSizes).Width(62).KeepFocus().Label("Size").Changed() {
			if px, err := strconv.ParseFloat(size, 32); err == nil {
				doc.Format(func(at *richtext.Attributes) { at.Size = float32(px) })
				a.saveSignature()
			}
		}

		a.signatureToolButton(c, "B", "Bold", f.Bold == richtext.FormatOn, doc.ToggleBold)
		a.signatureToolButton(c, "I", "Italic", f.Italic == richtext.FormatOn, doc.ToggleItalic)
		a.signatureToolButton(c, "U", "Underline", f.Underline == richtext.FormatOn, doc.ToggleUnderline)
		a.signatureToolButton(c, "S", "Strikethrough", f.Strikethrough == richtext.FormatOn, doc.ToggleStrikethrough)

		ui.Box(c).Width(1).Height(20).Background(p.Border)

		if f.Attributes.Color.A > 0 {
			ed.color = f.Attributes.Color
		}
		well := richColorWell(c, &ed.color, "Text color")
		if richColorWellChanged(well) {
			doc.Format(func(at *richtext.Attributes) { at.Color = ed.color })
			a.saveSignature()
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
			a.signatureToolButton(c, align.label, "Align "+align.label, on, func() {
				doc.SetAlign(align.value)
			})
		}

		ui.Box(c).Width(1).Height(20).Background(p.Border)

		if a.plainButton(c, "Insert image", "image").Clicked() {
			a.insertSignatureImage()
		}
		if a.ghostButton(c, "Undo", "").Clicked() && doc.CanUndo() {
			doc.Undo()
			a.saveSignature()
		}
		if a.ghostButton(c, "Redo", "").Clicked() && doc.CanRedo() {
			doc.Redo()
			a.saveSignature()
		}
	})

	// The editor: images inserted here land on the line where the caret is,
	// just like in a normal word processor.
	// The editor draws its own surface and border; this is only a layout
	// slot so the settings page shows one input, not a box inside a box.
	ui.Scroll(c).Fill().MaxHeight(320).Children(func() {
		richtext.Editor(c, doc, richtext.EditorOptions{
			Label:       "Signature",
			Placeholder: "Your name, role, phone or website",
			Font:        ui.Font{Size: 14},
			NoMarkdown:  true,
			Theme:       signatureRichTheme(c, p),
		}).FillWidth().Grow(1).MinHeight(100)
	})
	// Typing edits the document; keep the account in step so the
	// signature survives a restart without pressing a toolbar button.
	if v := doc.Version(); v != ed.version {
		a.saveSignature()
	}

	// Resize/remove the image the caret is on.
	if att := a.selectedSignatureAttachment(); att != nil {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Image selected").FontSize(10.5).TextColor(p.Muted)
			if a.ghostButton(c, "Smaller", "").Clicked() {
				a.resizeSelectedSignatureImage(-20)
			}
			if a.ghostButton(c, "Larger", "").Clicked() {
				a.resizeSelectedSignatureImage(20)
			}
			if a.ghostButton(c, "Remove", "trash").Clicked() {
				a.removeSelectedSignatureImage()
			}
		})
	}
	ui.Text(c, "The signature is sent with new messages, replies and forwards.").
		FontSize(9.5).TextColor(p.Muted)
}

// selectedSignatureAttachment reports the attachment under the caret, if any.
func (a *App) selectedSignatureAttachment() *richtext.Attachment {
	doc := a.signatureDoc().doc
	f := doc.Formatting()
	return f.Attributes.Attachment
}

// resizeSelectedSignatureImage changes the width of the selected image.
func (a *App) resizeSelectedSignatureImage(delta float32) {
	doc := a.signatureDoc().doc
	anchor, caret := doc.Selection()
	start, end := anchor, caret
	if start == end {
		if start > 0 {
			start--
		} else {
			return
		}
	}
	if start > end {
		start, end = end, start
	}
	doc.Format(func(at *richtext.Attributes) {
		if at.Attachment != nil {
			att := at.Attachment
			newW := att.Width + delta
			if newW < 20 {
				newW = 20
			}
			if newW > 600 {
				newW = 600
			}
			aspect := float32(1)
			if att.Width > 0 && att.Height > 0 {
				aspect = att.Height / att.Width
			}
			att.Width = newW
			att.Height = newW * aspect
		}
	})
	a.saveSignature()
	a.invalidate()
}

// removeSelectedSignatureImage deletes the selected image from the text.
func (a *App) removeSelectedSignatureImage() {
	doc := a.signatureDoc().doc
	anchor, caret := doc.Selection()
	start, end := anchor, caret
	if start == end {
		if start > 0 {
			start--
		} else {
			return
		}
	}
	if start > end {
		start, end = end, start
	}
	doc.Edit(func(e *richtext.Edit) { e.Replace(start, end, richtext.NewText("")) })
	a.saveSignature()
	a.invalidate()
}

// signatureToolButton draws one formatting control, highlighted when the
// selection has that format.
func (a *App) signatureToolButton(c *ui.Context, label, tip string, on bool, run func()) ui.Element {
	p := a.palette
	b := ui.Button(c, label).KeepFocus().Padding(4, 8).MinWidth(28).Radius(6).Tooltip(tip)
	if on {
		b.Background(p.Accent.Alpha(0.18))
		b.TextColor(p.Accent)
	}
	if b.Clicked() {
		run()
		a.saveSignature()
	}
	return b
}

func signatureRichTheme(c *ui.Context, p palette) *richtext.Theme {
	th := richtext.DefaultTheme(c.Theme())
	th.Text = p.Text
	th.Muted = p.Muted
	th.Link = p.Accent
	th.Caret = p.Text
	th.Selection = p.Accent.Alpha(0.25)
	th.Code = p.Text
	th.CodeBackground = p.Field
	th.CodeBlockBackground = p.Field
	th.QuoteBar = p.Border
	th.QuoteText = p.Muted
	th.Marker = p.Muted
	return th
}

// resolveSignatureCIDs replaces cid:signature-image-N references with data
// URLs, so images render in places that cannot resolve CIDs (the app's
// own reader web view).
func resolveSignatureCIDs(html string, a Account) string {
	if !strings.Contains(html, "cid:signature-image-") {
		return html
	}
	data, types := signatureImageParts(a)
	for i := range data {
		cid := fmt.Sprintf("cid:signature-image-%d", i)
		du := fmt.Sprintf("data:%s;base64,%s", types[i], base64.StdEncoding.EncodeToString(data[i]))
		html = strings.ReplaceAll(html, cid, du)
	}
	return html
}
