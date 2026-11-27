package app

import (
	"encoding/base64"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// pickSignatureImages adds pictures to the signature canvas. Their initial
// size is capped so a large photo does not cover the signature at once;
// the user can drag and resize them afterwards.
func (a *App) pickSignatureImages() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      a.mainWindow,
		Title:       "Choose signature images",
		ButtonLabel: "Choose",
		Filters:     []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp"}}},
		Multiple:    true,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	ed := a.signatureDoc()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) > 500*1024 {
			a.note = "A signature image is too large (max 500 KB)."
			continue
		}
		bmp, err := ui.DecodeBitmap(data)
		if err != nil {
			continue
		}
		iw, _ := bmp.Size()
		w := float32(iw)
		if w > 160 {
			w = 160
		}
		if w < 24 {
			w = 24
		}
		n := len(a.acct.SignatureCanvas)
		a.acct.SignatureCanvas = append(a.acct.SignatureCanvas, SignatureImage{
			Data:  base64.StdEncoding.EncodeToString(data),
			X:     6 + float32(n%3)*7,
			Y:     6 + float32(n%4)*6,
			Width: w / signatureCanvasW * 100,
		})
		ed.sel = len(a.acct.SignatureCanvas) - 1
	}
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active] = a.acct
	}
	a.saveConfig()
	a.invalidateAll()
}
