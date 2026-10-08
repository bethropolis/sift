package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// dataDir is the XDG user data dir (tests override via XDG_DATA_HOME/HOME).
func dataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot find a home directory (set $HOME or $XDG_DATA_HOME)")
	}
	return filepath.Join(home, ".local", "share"), nil
}

func entryPath() (string, error) {
	data, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "applications", "sift.desktop"), nil
}

func iconPath() (string, error) {
	data, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "icons", "hicolor", "512x512", "apps", "sift.png"), nil
}

// entryText builds the .desktop file. Exec is double-quoted so paths with
// spaces survive; the marker comment owns the file for uninstall.
func entryText(exePath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — remove with `sift desktop uninstall`\n", marker)
	b.WriteString(`[Desktop Entry]
Type=Application
Name=Sift
Comment=Sift: pack a codebase into LLM context
`)
	fmt.Fprintf(&b, "Exec=%q serve --app\n", exePath)
	b.WriteString(`Icon=sift
Terminal=false
Categories=Development;
Keywords=code;llm;context;
StartupNotify=false
`)
	return b.String()
}

// refreshCaches best-effort updates the desktop/icon caches; missing tools
// are fine (file managers fall back to reading the files directly).
func refreshCaches(entry, icon string) {
	if dir := filepath.Dir(entry); dir != "" {
		if p, err := exec.LookPath("update-desktop-database"); err == nil {
			_ = exec.Command(p, dir).Run()
		}
	}
	if p, err := exec.LookPath("gtk-update-icon-cache"); err == nil {
		_ = exec.Command(p, "-f", "-t", filepath.Dir(filepath.Dir(filepath.Dir(icon)))).Run()
	}
}

func installLinux() (Report, error) {
	exePath, err := exe()
	if err != nil {
		return Report{}, err
	}
	entry, err := entryPath()
	if err != nil {
		return Report{}, err
	}
	icon, err := iconPath()
	if err != nil {
		return Report{}, err
	}
	if _, statErr := os.Stat(entry); statErr == nil && !owned(entry) {
		return Report{}, &ErrForeign{Path: entry}
	}
	if err := writeFile(entry, []byte(entryText(exePath))); err != nil {
		return Report{}, fmt.Errorf("write %s: %w", entry, err)
	}
	if err := writeFile(icon, iconPNG); err != nil {
		return Report{}, fmt.Errorf("write %s: %w", icon, err)
	}
	refreshCaches(entry, icon)
	return Report{Files: []string{entry, icon}, Icon: true}, nil
}

func uninstallLinux() (Report, error) {
	entry, err := entryPath()
	if err != nil {
		return Report{}, err
	}
	icon, err := iconPath()
	if err != nil {
		return Report{}, err
	}
	var removed []string
	if _, statErr := os.Stat(entry); statErr == nil {
		if !owned(entry) {
			return Report{Files: removed}, &ErrForeign{Path: entry}
		}
		if err := os.Remove(entry); err != nil {
			return Report{}, fmt.Errorf("remove %s: %w", entry, err)
		}
		removed = append(removed, entry)
	}
	// The icon path is ours by name; remove it whenever present (a foreign
	// sift.png there would only exist if something else claimed our name).
	if _, statErr := os.Stat(icon); statErr == nil {
		if err := os.Remove(icon); err != nil {
			return Report{}, fmt.Errorf("remove %s: %w", icon, err)
		}
		removed = append(removed, icon)
	}
	refreshCaches(entry, icon)
	return Report{Files: removed, Icon: true}, nil
}

func statusLinux() Report {
	entry, _ := entryPath()
	icon, _ := iconPath()
	rep := Report{}
	if entry != "" {
		if _, err := os.Stat(entry); err == nil && owned(entry) {
			rep.Files = append(rep.Files, entry)
		}
	}
	if icon != "" {
		if _, err := os.Stat(icon); err == nil {
			rep.Files = append(rep.Files, icon)
			rep.Icon = true
		}
	}
	return rep
}
