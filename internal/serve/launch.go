// launch.go opens the serve UI in a chromeless app-mode window
// (Chromium `--app=`) for `sift serve --app`. Launching is best-effort by
// design: a missing browser or an unlaunchable process is never fatal, since
// the server stays useful through a normal browser or the printed URL.
//
// Every OS touchpoint (PATH lookup, stat, env, process start) is injected so
// discovery and argument building are pure and testable on any platform.
package serve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// appBrowserEnv overrides browser discovery with an explicit binary path.
const appBrowserEnv = "SIFT_APP_BROWSER"

// launcher holds the injected dependencies of app-mode discovery. Tests
// replace every field; production builds it with real implementations.
type launcher struct {
	goos     string
	lookPath func(string) (string, error)
	stat     func(string) (os.FileInfo, error)
	getenv   func(string) string
	start    func(path string, args []string) (*exec.Cmd, error)
	userHome func() (string, error)
}

func newLauncher() launcher {
	return launcher{
		goos:     runtime.GOOS,
		lookPath: exec.LookPath,
		stat:     os.Stat,
		getenv:   os.Getenv,
		start: func(path string, args []string) (*exec.Cmd, error) {
			// Stdin/stdout/stderr stay nil so the child gets /dev/null and
			// never competes for the terminal sift is running on.
			cmd := exec.Command(path, args...)
			return cmd, cmd.Start()
		},
		userHome: os.UserHomeDir,
	}
}

// linuxBrowsers are the Chromium-family binaries that support --app, in
// preference order.
var linuxBrowsers = []string{
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"brave-browser",
	"microsoft-edge",
	"microsoft-edge-stable",
}

// macBrowsers are absolute .app binary paths; the home-directory variants are
// appended at lookup time.
var macBrowsers = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
	"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
}

// findAppBrowser returns the first usable Chromium-family binary, or false.
// SIFT_APP_BROWSER wins when set and valid; a bad override is reported and
// ignored so discovery still has a chance to succeed.
func (l launcher) findAppBrowser() (path string, found bool, warned bool) {
	if override := strings.TrimSpace(l.getenv(appBrowserEnv)); override != "" {
		if l.usable(override) {
			return override, true, false
		}
		warned = true
	}
	switch l.goos {
	case "darwin":
		if p, ok := l.firstExisting(l.macCandidates()); ok {
			return p, true, warned
		}
	case "windows":
		if p, ok := l.firstExisting(l.windowsCandidates()); ok {
			return p, true, warned
		}
	default:
		// Linux, BSD and anything else use PATH lookup; Chromium ships there.
		for _, name := range linuxBrowsers {
			if p, err := l.lookPath(name); err == nil && p != "" {
				return p, true, warned
			}
		}
	}
	return "", false, warned
}

// usable reports whether path names an existing regular file (or symlink to
// one).
func (l launcher) usable(path string) bool {
	if l.stat == nil || path == "" {
		return false
	}
	info, err := l.stat(path)
	return err == nil && info != nil && info.Mode().IsRegular()
}

func (l launcher) firstExisting(paths []string) (string, bool) {
	for _, p := range paths {
		if l.usable(p) {
			return p, true
		}
	}
	return "", false
}

// macCandidates returns the .app paths plus their ~/Applications equivalents.
func (l launcher) macCandidates() []string {
	out := append([]string(nil), macBrowsers...)
	if l.userHome != nil {
		if home, err := l.userHome(); err == nil && home != "" {
			for _, p := range macBrowsers {
				out = append(out, filepath.Join(home, "Applications", strings.TrimPrefix(p, "/Applications/")))
			}
		}
	}
	return out
}

// windowsCandidates resolves the %ProgramFiles%-style locations. Empty
// environment variables are skipped so a bare "Google\Chrome\Application\"
// never probes the filesystem root.
func (l launcher) windowsCandidates() []string {
	var out []string
	add := func(env, rel string) {
		if base := strings.TrimSpace(l.getenv(env)); base != "" {
			out = append(out, filepath.Join(base, rel))
		}
	}
	for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
		add(env, `Google\Chrome\Application\chrome.exe`)
	}
	add("ProgramFiles(x86)", `Google\Chrome\Application\chrome.exe`)
	add("LocalAppData", `Google\Chrome\Application\chrome.exe`)
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "ProgramW6432"} {
		add(env, `Microsoft\Edge\Application\msedge.exe`)
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
		add(env, `BraveSoftware\Brave-Browser\Application\brave.exe`)
	}
	return out
}

// appProfileDir returns the dedicated browser profile directory for the app
// window. A separate profile keeps the window free of the user's everyday
// tabs and extensions, and stops Chromium from handing the URL to an already
// running instance and exiting immediately (which would be misread as the
// window closing).
func (l launcher) appProfileDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "sift", "app-profile")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// appArgs builds the Chromium argument vector. Flags are deliberately
// minimal: anything speculative breaks on some browser version. No
// --window-size, because the profile already remembers it.
func appArgs(url, profileDir string) []string {
	return []string{
		"--app=" + url,
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
}

// hasDisplay reports whether a GUI session is plausibly available. Covers the
// SSH-without-X case: there is no point spawning a browser that cannot open a
// window.
func (l launcher) hasDisplay() bool {
	if l.goos == "darwin" || l.goos == "windows" {
		return true
	}
	return strings.TrimSpace(l.getenv("DISPLAY")) != "" || strings.TrimSpace(l.getenv("WAYLAND_DISPLAY")) != ""
}

// launchApp starts the app-mode browser. It returns false (never an error
// beyond a reason string) so the caller can degrade to a normal browser or to
// just printing the URL. Nothing here can crash the server.
func (s *Server) launchApp(url string) (launched bool, reason string) {
	l := newLauncher()
	if !l.hasDisplay() {
		return false, "no DISPLAY or WAYLAND_DISPLAY; not launching a browser"
	}
	browser, found, warned := l.findAppBrowser()
	if warned {
		s.log.Warn(appBrowserEnv + " is set but not usable; falling back to discovery")
	}
	if !found {
		return false, "no Chromium-family browser found (set " + appBrowserEnv + " to override)"
	}
	profile, err := l.appProfileDir()
	if err != nil {
		return false, fmt.Sprintf("cannot create app profile: %v", err)
	}
	// The launch URL carries a short-lived single-use token and is visible in
	// the browser's argv; that is acceptable only because the token expires in
	// seconds and works once. Never log it.
	cmd, err := l.start(browser, appArgs(url, profile))
	if err != nil {
		return false, fmt.Sprintf("cannot start app window: %v", err)
	}
	if cmd != nil {
		// Reap the child so it never becomes a zombie. Its exit is NOT a
		// shutdown signal: a second launch with the same profile hands off to
		// the first instance and exits immediately.
		go func() {
			_ = cmd.Wait()
			s.log.Debug("app window process exited")
		}()
	}
	s.log.Info("launching app window")
	return true, ""
}
