package main

import (
	"encoding/json"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settingsData is the import/export home. Import auto-detects the client that
// wrote the file; export offers a full Go Post backup and a Postman collection
// that the other clients can read.
func (a *app) settingsData(c *ui.Context) {
	p := a.palette
	a.settingsSection(c, "Import", "Bring requests and environments in from Postman, Yaak, Insomnia, Hoppscotch, cURL, or a Go Post backup. Files are detected automatically.")
	ui.Row(c).FillWidth().Gap(9).Children(func() {
		imp := a.accentButton(c, "Import file…", "upload")
		if imp.Clicked() {
			a.importDataFile(c)
		}
		ui.Spacer(c).Grow(1)
	})
	ui.Row(c).FillWidth().Padding(9, 11).Gap(8).Radius(10).
		Background(p.Field).Border(1, p.Border).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon("file", 14)).Size(14, 14).TextColor(p.Muted)
		ui.Text(c, "Nothing is overwritten: imported requests and environments are added to the current workspace.").
			FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
	})

	a.settingsSection(c, "Export", "Save a copy of everything, or share your requests as a Postman v2.1 collection.")
	ui.Row(c).FillWidth().Gap(9).Children(func() {
		backup := a.plainButton(c, "Go Post backup", "download")
		if backup.Clicked() {
			a.exportBackupFile(c)
		}
		postman := a.plainButton(c, "Postman collection", "download")
		if postman.Clicked() {
			a.exportPostmanFile(c)
		}
	})
	ui.Row(c).FillWidth().Padding(9, 11).Gap(8).Radius(10).
		Background(p.Field).Border(1, p.Border).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon("folder", 14)).Size(14, 14).TextColor(p.Muted)
		ui.Text(c, "The backup restores workspaces, themes and settings. The Postman file carries the current workspace's requests and active variables.").
			FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
	})
}

func (a *app) importDataFile(c *ui.Context) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      a.settingsWin,
		Title:       "Import requests and environments",
		ButtonLabel: "Import",
		Filters: []mygo.FileFilter{{
			Name:       "HTTP client data",
			Extensions: []string{"json", "txt", "curl", "postman_collection"},
		}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	b, err := os.ReadFile(paths[0])
	if err != nil {
		a.toast(c, "Import failed", err.Error(), "error")
		return
	}
	a.importExchange(c, b)
}

// importExchange is also what tests call: it keeps the file dialog out of the
// parsing and applying path.
func (a *app) importExchange(c *ui.Context, data []byte) {
	format := importFormat(data)
	if format == "gopost" {
		var b backupFile
		if err := json.Unmarshal(data, &b); err != nil || len(b.Data.Workspaces) == 0 {
			a.toast(c, "Import failed", "This Go Post backup is not valid.", "error")
			return
		}
		a.openConfirm("Replace all data?", "A Go Post backup restores every workspace, theme and setting. Your current data will be replaced.", func() {
			a.data = b.Data
			if a.data.workspace(a.data.ActiveWorkspace) == nil {
				a.data.ActiveWorkspace = a.data.Workspaces[0].ID
			}
			if active := a.data.workspace(a.data.ActiveWorkspace); active != nil {
				a.data.Settings.Theme = active.Theme
				a.data.Settings.Material = active.Material
				a.data.Settings.Acrylic = active.Material == "acrylic"
			}
			a.save()
			a.refreshWindows()
			if c != nil {
				a.toast(c, "Backup restored", "Your Go Post data was restored.", "success")
			}
		})
		return
	}
	x, format, err := parseImport(data)
	if err != nil {
		a.toast(c, "Import failed", err.Error(), "error")
		return
	}
	collections, requests, envs := a.applyImport(x)
	a.save()
	a.refreshWindows()
	a.toast(c, "Import complete", importSummary(format, collections, requests, envs), "success")
}

func (a *app) exportBackupFile(c *ui.Context) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.settingsWin,
		Title:       "Export a Go Post backup",
		DefaultPath: "go-post-backup.json",
		ButtonLabel: "Export",
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || path == "" {
		return
	}
	b, err := backupJSON(a.data)
	if err == nil {
		err = os.WriteFile(path, b, 0o600)
	}
	if err != nil {
		a.toast(c, "Export failed", err.Error(), "error")
		return
	}
	a.toast(c, "Backup exported", "Your workspaces, themes and settings were saved.", "success")
}

func (a *app) exportPostmanFile(c *ui.Context) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.settingsWin,
		Title:       "Export a Postman collection",
		DefaultPath: "go-post-collection.json",
		ButtonLabel: "Export",
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || path == "" {
		return
	}
	b, err := postmanCollection(a.data)
	if err == nil {
		err = os.WriteFile(path, b, 0o600)
	}
	if err != nil {
		a.toast(c, "Export failed", err.Error(), "error")
		return
	}
	w := a.ws()
	name := "current workspace"
	if w != nil {
		name = w.Name
	}
	a.toast(c, "Collection exported", "Requests from "+name+" were saved as Postman v2.1.", "success")
}
