package gitclone

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	good := []string{
		"https://github.com/owner/repo",
		"https://github.com/owner/repo.git",
		"http://example.com/r.git",
		"ssh://git@github.com/owner/repo.git",
		"git://example.com/r.git",
		"git@github.com:owner/repo.git",
		"github.com:owner/repo",
		"  https://github.com/owner/repo  ",
		"https://token@github.com/owner/repo",
		"https://[::1]:8443/owner/repo",
	}
	for _, u := range good {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want ok", u, err)
		}
	}

	bad := map[string]string{
		"":                                   "empty",
		"   ":                                "blank",
		"/home/me/repo":                      "absolute path",
		"./repo":                             "relative path",
		"../repo":                            "parent path",
		"~/repo":                             "tilde path",
		"repo":                               "bare name",
		"file:///tmp/repo":                   "file scheme",
		"FILE:///tmp/repo":                   "file scheme case",
		"ext::sh -c 'touch /tmp/pwn'":        "ext helper",
		"EXT::sh -c id":                      "ext helper case",
		"hg::https://example.com/r":          "other helper",
		"-oProxyCommand=id":                  "option injection",
		"--upload-pack=id":                   "option injection long",
		"ssh://-oProxyCommand=id/x":          "dash host url",
		"-oProxyCommand=id@host:path":        "dash scp",
		"git@-host:path":                     "dash scp host",
		"https://user:secret@github.com/o/r": "embedded password",
		"ftp://example.com/r":                "unsupported scheme",
		"javascript://x":                     "bogus scheme",
		"https:///no-host":                   "no host",
		"https://github.com/o/r with space":  "space",
		"https://github.com/o/r\nfoo":        "newline",
		"https://github.com/o/r\x00":         "nul",
		"host:":                              "scp no path",
		":path":                              "scp no host",
		"a b@host:path":                      "scp space",
		"user@host/with/slash:path":          "slash before colon is a path",
		strings.Repeat("a", maxURLLen+1):     "too long",
		"http://example.com/r\x7f":           "del char",
	}
	for u, why := range bad {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) accepted (%s)", u, why)
		}
	}
}

func TestValidateRef(t *testing.T) {
	for _, ok := range []string{"", "main", "v1.2.3", "feature/x"} {
		if err := ValidateRef(ok); err != nil {
			t.Errorf("ValidateRef(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"-x", "--upload-pack=id", "a b", "a\nb", strings.Repeat("a", 256)} {
		if err := ValidateRef(bad); err == nil {
			t.Errorf("ValidateRef(%q) accepted", bad)
		}
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("https://user:pw@github.com/o/r"); strings.Contains(got, "pw") || strings.Contains(got, "user") {
		t.Errorf("Redact leaked userinfo: %q", got)
	}
	if got := Redact("git@github.com:o/r.git"); got != "git@github.com:o/r.git" {
		t.Errorf("scp URL changed: %q", got)
	}
}

func TestRepoName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/owner/repo.git": "repo",
		"https://github.com/owner/repo/":    "repo",
		"git@github.com:owner/my-repo.git":  "my-repo",
		"ssh://git@host/a/b/c":              "c",
		"https://host/..":                   "repo",
		"https://host/":                     "repo",
		"https://host/we ird$name":          "we_ird_name",
		"https://host/.hidden":              "hidden",
	}
	for in, want := range cases {
		if got := RepoName(in); got != want {
			t.Errorf("RepoName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestArgs(t *testing.T) {
	args := Args(Options{URL: "https://h/o/r", Branch: "dev", Depth: 3, Dest: "/tmp/x", OnProgress: func(Progress) {}})
	want := []string{
		"-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull,
		"clone", "--depth=3", "--single-branch", "--branch", "dev", "--progress",
		"--", "https://h/o/r", "/tmp/x",
	}
	if !slices.Equal(args, want) {
		t.Errorf("args = %q\nwant   %q", args, want)
	}
	// The URL always follows "--" so it can never be read as an option.
	if i := slices.Index(args, "--"); args[i+1] != "https://h/o/r" {
		t.Errorf("url not after --: %q", args)
	}
	if slices.Contains(Args(Options{URL: "https://h/o/r", Depth: 1, Dest: "/x"}), "--progress") {
		t.Error("--progress without an OnProgress consumer")
	}
}

func TestNewTemp(t *testing.T) {
	tmp, err := NewTemp("repo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Base(tmp.Dir), "sift-clone-") {
		t.Errorf("unexpected temp dir %q", tmp.Dir)
	}
	if filepath.Dir(tmp.Repo) != tmp.Dir {
		t.Errorf("repo %q not inside %q", tmp.Repo, tmp.Dir)
	}
	if err := tmp.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmp.Dir); !os.IsNotExist(err) {
		t.Errorf("temp dir survived Remove: %v", err)
	}
	if err := tmp.Remove(); err != nil {
		t.Errorf("second Remove should be a no-op, got %v", err)
	}
}

// fakeGit puts a stand-in `git` first on PATH. The stand-in records its
// environment into the last argument's path (the destination) and then runs
// the supplied shell body.
func fakeGit(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-in needs a POSIX sh")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"for a; do last=$a; done\n" +
		"printf '%s\\n%s\\n' \"$GIT_TERMINAL_PROMPT\" \"$GIT_ALLOW_PROTOCOL\" > \"$last.env\"\n" +
		body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCloneProgressAndEnv(t *testing.T) {
	fakeGit(t, `printf 'Cloning into x...\nremote: Counting objects: 100%% (5/5), done.\rReceiving objects:  42%% (2/5)\rReceiving objects: 100%% (5/5), done.\n' >&2`)
	dest := filepath.Join(t.TempDir(), "repo")
	var got []Progress
	var raw bytes.Buffer
	err := Clone(context.Background(), Options{
		URL: "https://example.com/o/r", Depth: 1, Dest: dest, Output: &raw,
		OnProgress: func(p Progress) { got = append(got, p) },
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	want := []Progress{{"Counting objects", 100}, {"Receiving objects", 42}, {"Receiving objects", 100}}
	if !slices.Equal(got, want) {
		t.Errorf("progress = %v, want %v", got, want)
	}
	if !strings.Contains(raw.String(), "Cloning into x") {
		t.Errorf("Output writer did not receive git's stream: %q", raw.String())
	}
	env, _ := os.ReadFile(dest + ".env")
	if string(env) != "0\n"+allowedProtocols+"\n" {
		t.Errorf("non-interactive env = %q", env)
	}
}

func TestCloneInteractiveKeepsPrompts(t *testing.T) {
	fakeGit(t, `exit 0`)
	dest := filepath.Join(t.TempDir(), "repo")
	if err := Clone(context.Background(), Options{URL: "https://example.com/o/r", Depth: 1, Dest: dest, Interactive: true}); err != nil {
		t.Fatal(err)
	}
	env, _ := os.ReadFile(dest + ".env")
	if strings.HasPrefix(string(env), "0\n") {
		t.Errorf("interactive clone must not disable terminal prompts: %q", env)
	}
}

func TestCloneFailureCarriesDetail(t *testing.T) {
	fakeGit(t, `printf 'fatal: could not read Username for https://example.com\n' >&2; exit 128`)
	err := Clone(context.Background(), Options{URL: "https://example.com/o/r", Depth: 1, Dest: filepath.Join(t.TempDir(), "r")})
	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("want *Failure, got %T %v", err, err)
	}
	if !strings.Contains(f.Detail, "could not read Username") {
		t.Errorf("detail = %q", f.Detail)
	}
	if strings.Contains(f.Error(), "Username") {
		t.Errorf("Error() should stay terse, got %q", f.Error())
	}
}

func TestCloneCancelKillsGit(t *testing.T) {
	fakeGit(t, `exec sleep 30`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Clone(ctx, Options{URL: "https://example.com/o/r", Depth: 1, Dest: filepath.Join(t.TempDir(), "r")})
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("want context.Canceled, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Clone did not return after cancel")
	}
}

func TestCloneRejectsBeforeRunningGit(t *testing.T) {
	fakeGit(t, `touch "$last.ran"`)
	dest := filepath.Join(t.TempDir(), "r")
	for _, o := range []Options{
		{URL: "/etc", Depth: 1, Dest: dest},
		{URL: "ext::sh -c id", Depth: 1, Dest: dest},
		{URL: "https://example.com/o/r", Branch: "-x", Depth: 1, Dest: dest},
		{URL: "https://example.com/o/r", Depth: 0, Dest: dest},
		{URL: "https://example.com/o/r", Depth: 1},
	} {
		if err := Clone(context.Background(), o); err == nil {
			t.Errorf("Clone(%+v) accepted", o)
		}
	}
	if _, err := os.Stat(dest + ".ran"); err == nil {
		t.Error("git ran for a rejected request")
	}
}

func TestCloneNoGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if Available() {
		t.Skip("git still resolvable with an empty PATH")
	}
	err := Clone(context.Background(), Options{URL: "https://example.com/o/r", Depth: 1, Dest: "/tmp/x"})
	if !errors.Is(err, ErrNoGit) {
		t.Errorf("want ErrNoGit, got %v", err)
	}
}
