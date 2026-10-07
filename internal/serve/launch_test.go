package serve

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeFileInfo reports a regular file so launcher.usable can be exercised
// without touching the filesystem.
type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string     { return f.name }
func (fakeFileInfo) Size() int64        { return 1 }
func (fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func newTestLauncher(existing map[string]bool, env map[string]string, look map[string]string) launcher {
	l := launcher{
		goos:     runtime.GOOS,
		getenv:   func(k string) string { return env[k] },
		lookPath: func(name string) (string, error) { return look[name], nil },
		stat: func(path string) (os.FileInfo, error) {
			if existing[path] {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			return nil, os.ErrNotExist
		},
		userHome: func() (string, error) { return "/home/tester", nil },
	}
	return l
}

func TestFindAppBrowserOverride(t *testing.T) {
	// Override wins when it points at a real file.
	l := newTestLauncher(
		map[string]bool{"/custom/chrome": true},
		map[string]string{appBrowserEnv: "/custom/chrome"},
		map[string]string{},
	)
	if p, ok, warned := l.findAppBrowser(); !ok || p != "/custom/chrome" || warned {
		t.Fatalf("override = %q/%v/%v", p, ok, warned)
	}

	// A bad override warns and falls through to discovery.
	l = newTestLauncher(
		map[string]bool{"/usr/bin/chromium": true},
		map[string]string{appBrowserEnv: "/nope/chrome"},
		map[string]string{"chromium": "/usr/bin/chromium"},
	)
	p, ok, warned := l.findAppBrowser()
	if !ok || p != "/usr/bin/chromium" || !warned {
		t.Fatalf("bad override = %q/%v/%v, want fallback + warning", p, ok, warned)
	}

	// Nothing found anywhere.
	l = newTestLauncher(nil, map[string]string{}, map[string]string{})
	if _, ok, _ := l.findAppBrowser(); ok {
		t.Fatal("empty environment reported a browser")
	}
}

func TestFindAppBrowserLinuxOrder(t *testing.T) {
	look := map[string]string{
		"chromium":             "/usr/bin/chromium",
		"google-chrome":        "/usr/bin/google-chrome",
		"microsoft-edge":       "/usr/bin/microsoft-edge",
		"brave-browser":        "/usr/bin/brave-browser",
		"google-chrome-stable": "/usr/bin/google-chrome-stable",
	}
	l := newTestLauncher(nil, map[string]string{}, look)
	l.goos = "linux"
	p, ok, _ := l.findAppBrowser()
	if !ok || p != "/usr/bin/google-chrome" {
		t.Fatalf("first-hit order = %q, want google-chrome", p)
	}
}

func TestFindAppBrowserWindowsSkipsEmptyEnv(t *testing.T) {
	l := launcher{
		goos: "windows",
		getenv: func(k string) string {
			// ProgramFiles(x86) deliberately unset.
			if k == "ProgramFiles(x86)" {
				return ""
			}
			return `C:\` + k
		},
		stat: func(path string) (os.FileInfo, error) {
			if strings.Contains(path, "chrome.exe") {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
	}
	got := l.windowsCandidates()
	for _, p := range got {
		if strings.HasPrefix(p, `C:\ProgramFiles(x86)`) && strings.Contains(p, "Edge") {
			t.Fatalf("edge candidate built from empty env var: %q", p)
		}
	}
	p, ok, _ := l.findAppBrowser()
	if !ok || !strings.HasSuffix(p, "chrome.exe") {
		t.Fatalf("windows discovery = %q/%v", p, ok)
	}
}

func TestAppArgs(t *testing.T) {
	// A path with spaces must survive as one argv element.
	got := appArgs("http://127.0.0.1:7777/#launch=abc", "/home/a b/sift/app-profile")
	want := []string{
		"--app=http://127.0.0.1:7777/#launch=abc",
		"--user-data-dir=/home/a b/sift/app-profile",
		"--no-first-run",
		"--no-default-browser-check",
	}
	if len(got) != len(want) {
		t.Fatalf("args = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// The fragment must survive intact.
	if !strings.Contains(got[0], "#launch=abc") {
		t.Fatalf("fragment lost: %q", got[0])
	}
	// No speculative window flags.
	for _, a := range got {
		if strings.Contains(a, "--window-size") {
			t.Fatalf("unexpected window flag: %q", a)
		}
	}
}

func TestHasDisplay(t *testing.T) {
	l := launcher{goos: "linux", getenv: func(string) string { return "" }}
	if l.hasDisplay() {
		t.Fatal("headless linux reported a display")
	}
	l.getenv = func(k string) string {
		if k == "WAYLAND_DISPLAY" {
			return "wayland-0"
		}
		return ""
	}
	if !l.hasDisplay() {
		t.Fatal("wayland display not detected")
	}
	// macOS/Windows never consult the env vars.
	l = launcher{goos: "darwin", getenv: func(string) string { return "" }}
	if !l.hasDisplay() {
		t.Fatal("darwin reported headless")
	}
}

func TestMacCandidatesIncludeHome(t *testing.T) {
	l := newTestLauncher(nil, map[string]string{}, map[string]string{})
	l.goos = "darwin"
	got := l.macCandidates()
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "/Applications/Google Chrome.app") {
		t.Fatalf("missing system path: %v", got)
	}
	if !strings.Contains(joined, "/home/tester/Applications/Brave Browser.app") {
		t.Fatalf("missing ~/Applications path: %v", got)
	}
}
