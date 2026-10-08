package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const bundleID = "com.bethropolis.sift"

func appDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot find a home directory (set $HOME)")
	}
	return filepath.Join(home, "Applications", "Sift.app"), nil
}

func plistPath(app string) string    { return filepath.Join(app, "Contents", "Info.plist") }
func launcherPath(app string) string { return filepath.Join(app, "Contents", "MacOS", "Sift") }
func iconICNSPath(app string) string { return filepath.Join(app, "Contents", "Resources", "Sift.icns") }

// launcherScript execs the installed binary. Single-quote wrapping with
// embedded-quote escaping keeps paths with spaces (and quotes) intact.
func launcherScript(exePath string) string {
	quoted := "'" + strings.ReplaceAll(exePath, "'", `'\''`) + "'"
	return "#!/bin/sh\n# " + marker + " — remove with `sift desktop uninstall`\n" +
		"exec " + quoted + " serve --app \"$@\"\n"
}

// plistText builds the bundle Info.plist. The XML comment owns the bundle
// for uninstall.
func plistText(version string) string {
	if version == "" {
		version = "0.0.0"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<!-- ` + marker + ` — remove with ` + "`sift desktop uninstall`" + ` -->
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>Sift</string>
	<key>CFBundleIdentifier</key>
	<string>` + bundleID + `</string>
	<key>CFBundleName</key>
	<string>Sift</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>` + plistEscape(version) + `</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
`)
	return b.String()
}

func plistEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// makeICNS converts the embedded PNG with sips (ships with macOS). False
// means "install without an icon", never a failure: the bundle still works.
func makeICNS(dest string) bool {
	sips, err := exec.LookPath("sips")
	if err != nil {
		return false
	}
	stage, err := os.CreateTemp("", "sift-icon-*.png")
	if err != nil {
		return false
	}
	defer func() { _ = os.Remove(stage.Name()) }()
	if _, err := stage.Write(iconPNG); err != nil {
		_ = stage.Close()
		return false
	}
	if err := stage.Close(); err != nil {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false
	}
	return exec.Command(sips, "-s", "format", "icns", stage.Name(), "--out", dest).Run() == nil
}

func installDarwin(version string) (Report, error) {
	exePath, err := exe()
	if err != nil {
		return Report{}, err
	}
	app, err := appDir()
	if err != nil {
		return Report{}, err
	}
	plist := plistPath(app)
	if _, statErr := os.Stat(plist); statErr == nil && !owned(plist) {
		return Report{}, &ErrForeign{Path: app}
	}
	launcher := launcherPath(app)
	if err := writeFile(plist, []byte(plistText(version))); err != nil {
		return Report{}, fmt.Errorf("write %s: %w", plist, err)
	}
	if err := writeFile(launcher, []byte(launcherScript(exePath))); err != nil {
		return Report{}, fmt.Errorf("write %s: %w", launcher, err)
	}
	if err := os.Chmod(launcher, 0o755); err != nil {
		return Report{}, fmt.Errorf("chmod %s: %w", launcher, err)
	}
	rep := Report{Files: []string{plist, launcher}}
	if makeICNS(iconICNSPath(app)) {
		rep.Files = append(rep.Files, iconICNSPath(app))
		rep.Icon = true
	}
	return rep, nil
}

func uninstallDarwin() (Report, error) {
	app, err := appDir()
	if err != nil {
		return Report{}, err
	}
	if _, statErr := os.Stat(plistPath(app)); statErr != nil {
		return Report{}, nil // nothing there: success
	}
	if !owned(plistPath(app)) {
		return Report{}, &ErrForeign{Path: app}
	}
	if err := os.RemoveAll(app); err != nil {
		return Report{}, fmt.Errorf("remove %s: %w", app, err)
	}
	return Report{Files: []string{app}}, nil
}

func statusDarwin() Report {
	app, err := appDir()
	if err != nil {
		return Report{}
	}
	if _, statErr := os.Stat(plistPath(app)); statErr != nil {
		return Report{}
	}
	if !owned(plistPath(app)) {
		return Report{}
	}
	rep := Report{Files: []string{app}}
	if _, err := os.Stat(iconICNSPath(app)); err == nil {
		rep.Icon = true
	}
	return rep
}
