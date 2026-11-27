package mail

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

type ThemeSettings struct {
	Theme    string  `json:"theme"`
	Accent   string  `json:"accent"`
	Compact  bool    `json:"compact"`
	FontSize float32 `json:"fontSize"`
	Radius   int     `json:"radius"`
	Material string  `json:"material,omitempty"`
	Debug    bool    `json:"debug,omitempty"`

	// Legacy fields kept so older configs keep loading.
	Preset string `json:"preset,omitempty"`
	Dark   bool   `json:"dark,omitempty"`
}

func RenumberSignatureAttachments(text richtext.Text, imgs *[]string) richtext.Text {
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

// RichSignatureHTML returns the signature's rich text, without the meta
// tag Text.HTML adds, falling back to HTML or plain text saved by older
// versions.
func RichSignatureHTML(a Account) string {
	if rich := strings.TrimSpace(a.SignatureRich); rich != "" {
		var text richtext.Text
		if err := json.Unmarshal([]byte(rich), &text); err == nil {
			text = RenumberSignatureAttachments(text, &[]string{})
			return strings.TrimPrefix(text.HTML(), `<meta charset="utf-8">`)
		}
	}
	if legacy := strings.TrimSpace(a.SignatureHTML); legacy != "" {
		return strings.TrimPrefix(legacy, `<meta charset="utf-8">`)
	}
	if strings.TrimSpace(a.Signature) == "" {
		return ""
	}
	return TextToHTML(strings.TrimSpace(a.Signature))
}

// SignatureHTML renders the rich signature. Images are inline in the rich
// text now; legacy canvas images are appended for old signatures.
func SignatureHTML(a Account) string {
	rich := RichSignatureHTML(a)
	if !strings.Contains(rich, "<img") && len(a.SignatureCanvas) > 0 {
		var imgs strings.Builder
		idx := InlineImageCount(a)
		for _, im := range a.SignatureCanvas {
			raw, _ := DecodeSignatureImage(im.Data)
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

func InlineImageCount(a Account) int {
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

func LegacySignatureImages(a Account) []string {
	if len(a.SignatureImages) > 0 {
		return a.SignatureImages
	}
	if a.SignatureImage != "" {
		return []string{a.SignatureImage}
	}
	return nil
}

// ComposeHTML turns the composed message into its HTML part, replacing the
// plain signature the body carries with the rich one. It reports whether
// the rich signature was used, which needs a multipart message even
// without images or files.
func ComposeHTML(a Account, body string) (string, bool) {
	rich := SignatureHTML(a)
	sig := strings.TrimSpace(a.Signature)
	if rich != "" && sig != "" {
		if i := strings.Index(body, sig); i >= 0 {
			return TextToHTML(body[:i]) + rich + TextToHTML(body[i+len(sig):]), true
		}
	}
	if rich != "" && sig == "" {
		return TextToHTML(body) + "<br>" + rich, true
	}
	return TextToHTML(body), false
}
