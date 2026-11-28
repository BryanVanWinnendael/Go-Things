//go:build windows

package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"go-hub/internal/catalog"
)

// The setup mygo builds is NSIS: a per-user install that registers itself
// under HKCU\...\Uninstall\<identifier> and writes an Uninstall.exe. The hub
// reads exactly those values, so what it shows is what Windows knows.

var regValue = regexp.MustCompile(`^\s+(\S+)\s+(REG_[A-Z_]+)\s+(.*)$`)

func uninstallKey(app catalog.App) string {
	return `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\` + app.Identifier
}

// hidden keeps a console child such as reg.exe from flashing a window.
func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func ReadInstalled(app catalog.App) (Info, bool) {
	out, err := hidden(exec.Command("reg", "query", uninstallKey(app))).Output()
	if err != nil {
		return Info{}, false
	}
	info := Info{}
	for _, line := range strings.Split(string(out), "\n") {
		m := regValue.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[3])
		switch strings.ToLower(m[1]) {
		case "displayversion":
			info.Version = value
		case "installlocation":
			info.Location = strings.Trim(value, `"`)
		case "uninstallstring":
			info.Uninstall = value
		case "displayicon":
			info.Icon = value
		}
	}
	if info.Version == "" && info.Location == "" && info.Uninstall == "" {
		return Info{}, false
	}
	return info, true
}

// install.RunSetup installs the app with NSIS's silent switch, in the folder the
// setup was shipped in, so it finds any files beside it.
func RunSetup(path string) error {
	cmd := hidden(exec.Command(path, "/S"))
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Could not start the setup: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("The setup stopped with an error: %w", err)
	}
	return nil
}

// StartSetup starts a setup without waiting for it. The NSIS setup closes the
// running app itself before replacing its files, so a self-update has to let
// the installer outlive this process.
func StartSetup(path string) error {
	cmd := hidden(exec.Command(path, "/S"))
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Could not start the Go Hub update: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

// setupGuard recovers from a setup that cannot start at all.
// install.RunUninstaller prefers the quiet uninstall string the setup registers,
// which already carries /S, and falls back to adding it.
func RunUninstaller(info Info) error {
	if strings.TrimSpace(info.Uninstall) == "" {
		return errors.New("Windows does not list an uninstaller for this app. Remove it from Settings › Apps.")
	}
	exe, args := splitCommand(info.Uninstall)
	if !hasFlag(args, "/S") {
		args = append(args, "/S")
	}
	cmd := hidden(exec.Command(exe, args...))
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Could not start the uninstaller: %w", err)
	}
	// The NSIS uninstaller copies itself, starts the copy and exits; the
	// registry poll in install.WaitGone follows the copy.
	_ = cmd.Wait()
	return nil
}

func LaunchApp(app catalog.App, info Info) error {
	exe, err := appExecutable(app, info)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Could not open %s: %w", app.Name, err)
	}
	_ = cmd.Process.Release()
	return nil
}

// appExecutable is the installed program: the registered display icon
// first, then the executable in the registered install folder.
func appExecutable(app catalog.App, info Info) (string, error) {
	var candidates []string
	if info.Icon != "" {
		if path, _ := splitCommand(info.Icon); path != "" {
			candidates = append(candidates, path)
		}
	}
	if info.Location != "" {
		candidates = append(candidates, filepath.Join(info.Location, app.ExeName))
	}
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s is listed as installed, but its program is missing. Uninstall it and install it again.", app.Name)
}

// splitCommand splits a registry command such as
// "C:\Program Files\App\Uninstall.exe" /S into its program and arguments.
func splitCommand(s string) (string, []string) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `"`) {
		if i := strings.Index(s[1:], `"`); i >= 0 {
			return s[1 : i+1], strings.Fields(s[i+2:])
		}
		return strings.TrimPrefix(s, `"`), nil
	}
	// NSIS always writes quotes, but a path without spaces still parses.
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], strings.Fields(s[i+1:])
	}
	return s, nil
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if strings.EqualFold(a, flag) {
			return true
		}
	}
	return false
}
