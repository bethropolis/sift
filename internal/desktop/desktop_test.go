package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// asForeign reports whether err wraps an *ErrForeign.
func asForeign(err error) bool {
	var target *ErrForeign
	return errors.As(err, &target)
}

// isolateHome points HOME (and clears XDG_DATA_HOME) at a temp dir so both
// the Linux and macOS paths stay inside the test sandbox.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	return home
}

func TestEntryText(t *testing.T) {
	text := entryText("/opt/my apps/sift")
	for _, want := range []string{
		marker,
		`Exec="/opt/my apps/sift" serve --app`,
		"Icon=sift",
		"Terminal=false",
		"[Desktop Entry]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("entry text missing %q:\n%s", want, text)
		}
	}
}

func TestLinuxRoundTrip(t *testing.T) {
	isolateHome(t)
	rep, err := installLinux()
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if len(rep.Files) != 2 || !rep.Icon {
		t.Fatalf("report = %+v", rep)
	}
	for _, f := range rep.Files {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("missing installed file %s: %v", f, err)
		}
	}
	if _, err := installLinux(); err != nil {
		t.Fatalf("re-install: %v", err)
	}
	if got := statusLinux(); len(got.Files) != 2 {
		t.Errorf("status files = %v", got.Files)
	}
	out, err := uninstallLinux()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(out.Files) != 2 {
		t.Errorf("removed = %v", out.Files)
	}
	for _, f := range rep.Files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s survived uninstall", f)
		}
	}
	if out, err := uninstallLinux(); err != nil || len(out.Files) != 0 {
		t.Errorf("second uninstall = %v, %v", out, err)
	}
	if got := statusLinux(); len(got.Files) != 0 {
		t.Errorf("status after uninstall = %v", got.Files)
	}
}

func TestLinuxRefusesForeignEntry(t *testing.T) {
	isolateHome(t)
	entry, err := entryPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFile(entry, []byte("[Desktop Entry]\nName=Evil\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := installLinux(); !asForeign(err) {
		t.Errorf("install over foreign entry = %v, want ErrForeign", err)
	}
	if _, err := uninstallLinux(); !asForeign(err) {
		t.Errorf("uninstall of foreign entry = %v, want ErrForeign", err)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Errorf("foreign entry touched: %v", err)
	}
}

func TestLauncherScriptQuoting(t *testing.T) {
	script := launcherScript(`/home/me/my apps/si'ft`)
	if !strings.Contains(script, marker) {
		t.Error("script missing marker")
	}
	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Error("script missing shebang")
	}
	if !strings.Contains(script, `serve --app "$@"`) {
		t.Errorf("script missing exec line:\n%s", script)
	}
}

func TestPlistText(t *testing.T) {
	text := plistText("1.2.3")
	for _, want := range []string{marker, bundleID, "<string>1.2.3</string>", "CFBundleExecutable"} {
		if !strings.Contains(text, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	if got := plistText("1<2&3"); !strings.Contains(got, "1&lt;2&amp;3") {
		t.Errorf("version not XML-escaped:\n%s", got)
	}
}

func TestDarwinRoundTrip(t *testing.T) {
	isolateHome(t)
	rep, err := installDarwin("1.2.3")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if rep.Icon {
		t.Log("sips present: icon installed")
	}
	app, err := appDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{plistPath(app), launcherPath(app)} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("missing bundle file %s: %v", f, err)
		}
	}
	info, _ := os.Stat(launcherPath(app))
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("launcher is not executable")
	}
	if got := statusDarwin(); len(got.Files) != 1 {
		t.Errorf("status files = %v", got.Files)
	}
	out, err := uninstallDarwin()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(out.Files) != 1 || out.Files[0] != app {
		t.Errorf("removed = %v", out.Files)
	}
	if _, err := os.Stat(app); !os.IsNotExist(err) {
		t.Errorf("bundle survived uninstall: %v", err)
	}
	if out, err := uninstallDarwin(); err != nil || len(out.Files) != 0 {
		t.Errorf("second uninstall = %v, %v", out, err)
	}
}

func TestDarwinRefusesForeignBundle(t *testing.T) {
	isolateHome(t)
	app, err := appDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFile(plistPath(app), []byte(`<plist><dict></dict></plist>`)); err != nil {
		t.Fatal(err)
	}
	if _, err := installDarwin("1.0"); !asForeign(err) {
		t.Errorf("install over foreign bundle = %v, want ErrForeign", err)
	}
	if _, err := uninstallDarwin(); !asForeign(err) {
		t.Errorf("uninstall of foreign bundle = %v, want ErrForeign", err)
	}
	if _, err := os.Stat(filepath.Join(app)); err != nil {
		t.Errorf("foreign bundle touched: %v", err)
	}
}

func TestErrForeignMessage(t *testing.T) {
	err := &ErrForeign{Path: "/x"}
	if !strings.Contains(err.Error(), "/x") {
		t.Errorf("message = %q", err)
	}
}
