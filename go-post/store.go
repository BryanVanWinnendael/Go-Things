package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

// Store owns the JSON file and the files beside it. Views only touch the
// app's own copy; Save is called on the UI thread after a change.
type Store struct {
	mu       sync.Mutex
	dir      string
	file     string
	fallback map[string]string
}

func OpenStore() (*Store, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		if d, e := os.UserConfigDir(); e == nil {
			dir = filepath.Join(d, "go-post")
		} else {
			return nil, err
		}
	}
	s := &Store{dir: dir, file: filepath.Join(dir, "gopost.json"), fallback: map[string]string{}}
	if err := os.MkdirAll(filepath.Join(dir, "certificates"), 0o755); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "passwords.json"))
	if err == nil {
		_ = json.Unmarshal(b, &s.fallback)
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Load() (Data, error) {
	b, err := os.ReadFile(s.file)
	if errors.Is(err, os.ErrNotExist) {
		return defaultData(), nil
	}
	if err != nil {
		return Data{}, err
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		_ = os.WriteFile(s.file+".broken", b, 0o600)
		return defaultData(), err
	}
	if len(d.Workspaces) == 0 {
		d = defaultData()
	}
	if d.Settings.Theme == "" {
		d.Settings = defaultData().Settings
	}
	if d.Settings.TimeoutMS <= 0 {
		d.Settings.TimeoutMS = 30000
	}
	if d.Settings.FontSize <= 0 {
		d.Settings.FontSize = 14
	}
	// Before folder authorization existed, "none" was simply the default. Give
	// those requests the folder's authorization as Postman does, while choices
	// made after this version stay explicit.
	if d.Version < 2 {
		for wi := range d.Workspaces {
			ws := &d.Workspaces[wi]
			for ci := range ws.Collections {
				for ri := range ws.Collections[ci].Requests {
					r := &ws.Collections[ci].Requests[ri]
					if r.AuthType == "" || r.AuthType == "none" {
						r.AuthType = "inherit"
					}
				}
			}
		}
	}
	d.Version = 2
	// Migrate old Acrylic bool to Material string.
	if d.Settings.Material == "" {
		if d.Settings.Acrylic {
			d.Settings.Material = "acrylic"
		} else {
			d.Settings.Material = "none"
		}
	}
	// Themes used to be one global choice. Give every workspace that look the
	// first time it loads, then let each keep its own from here on. A theme
	// that was deleted while the file was closed falls back to System.
	for i := range d.Workspaces {
		ws := &d.Workspaces[i]
		if ws.Theme == "" {
			ws.Theme = d.Settings.Theme
		}
		if ws.Material == "" {
			ws.Material = d.Settings.Material
		}
		if !validTheme(ws.Theme, d.CustomThemes) {
			ws.Theme = "system"
		}
		ws.ensureTabs()
		// Requests saved before the Params tab kept their query in the URL.
		// Split it into rows so both sides show the same parameters.
		for ci := range ws.Collections {
			for ri := range ws.Collections[ci].Requests {
				absorbInlineQuery(&ws.Collections[ci].Requests[ri])
			}
		}
	}
	if d.workspace(d.ActiveWorkspace) == nil {
		d.ActiveWorkspace = d.Workspaces[0].ID
	}
	// Settings mirrors the active workspace, so everything that reads it sees
	// that workspace's theme rather than a stale global one.
	if active := d.workspace(d.ActiveWorkspace); active != nil {
		d.Settings.Theme = active.Theme
		d.Settings.Material = active.Material
		d.Settings.Acrylic = active.Material == "acrylic"
	}
	return d, nil
}

func (s *Store) Save(d Data) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.file)
}

func (s *Store) CertDir() string { return filepath.Join(s.dir, "certificates") }

// SavePassword keeps PFX passwords in the OS credential store when it is
// available, and in a user-only file when it is not (Linux without a secret
// service, for example).
func (s *Store) SavePassword(id, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := mygo.Secrets.Set("certificate/"+id, []byte(password)); err == nil {
		delete(s.fallback, id)
		return s.saveFallback()
	}
	s.fallback[id] = password
	return s.saveFallback()
}

func (s *Store) Password(id string) (string, error) {
	if b, err := mygo.Secrets.Get("certificate/" + id); err == nil {
		return string(b), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.fallback[id]; ok {
		return p, nil
	}
	return "", errors.New("password not stored")
}

func (s *Store) DeletePassword(id string) {
	_ = mygo.Secrets.Delete("certificate/" + id)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.fallback, id)
	_ = s.saveFallback()
}

func (s *Store) saveFallback() error {
	b, err := json.Marshal(s.fallback)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "passwords.json"), b, 0o600)
}
