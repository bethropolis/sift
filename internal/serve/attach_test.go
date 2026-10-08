package serve

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
)

func addrInUseErr() error {
	return fmt.Errorf("listen: %w", syscall.EADDRINUSE)
}

func TestIsAddrInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, err = net.Listen("tcp", ln.Addr().String())
	if err == nil {
		t.Fatal("second bind unexpectedly succeeded")
	}
	if !isAddrInUse(err) {
		t.Fatalf("conflict not detected: %v", err)
	}
	if isAddrInUse(fmt.Errorf("boom")) {
		t.Fatal("generic error misdetected as addr-in-use")
	}
}

func stubAddr(t *testing.T, body string, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/meta" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestProbeAcceptsSiftShape(t *testing.T) {
	addr := stubAddr(t, `{"version":"dev","mode":"local","authKind":"token","authenticated":false}`, http.StatusOK)
	scheme, ok := probeSiftServer(addr)
	if !ok || scheme != "http" {
		t.Fatalf("sift meta not accepted: scheme=%q ok=%v", scheme, ok)
	}
}

func TestProbeRejectsImpostors(t *testing.T) {
	cases := map[string]struct {
		body   string
		status int
	}{
		"wrong shape":     {`{"ok":true}`, http.StatusOK},
		"missing auth":    {`{"version":"x","mode":"local"}`, http.StatusOK},
		"missing version": {`{"mode":"local","authKind":"token"}`, http.StatusOK},
		"unknown mode":    {`{"version":"x","mode":"mars","authKind":"token"}`, http.StatusOK},
		"not json":        {`hello`, http.StatusOK},
		"empty json":      {`{}`, http.StatusOK},
		"server error":    {`{"version":"x","mode":"local","authKind":"token"}`, http.StatusInternalServerError},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := probeSiftServer(stubAddr(t, c.body, c.status)); ok {
				t.Fatal("impostor meta accepted")
			}
		})
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().String()
	ln.Close()
	if _, ok := probeSiftServer(dead); ok {
		t.Fatal("dead port accepted")
	}
}

func TestMaybeAttachSkipsWithoutWindowFlag(t *testing.T) {
	s := &Server{cfg: &Config{Listen: "127.0.0.1:7777"}}
	if s.maybeAttach(addrInUseErr()) {
		t.Fatal("plain serve attached without --open/--app")
	}
}

func TestMaybeAttachSkipsRemote(t *testing.T) {
	s := &Server{cfg: &Config{Listen: "0.0.0.0:7777", Open: true, App: true}}
	if s.maybeAttach(addrInUseErr()) {
		t.Fatal("remote bind attached")
	}
}

func TestMaybeAttachSkipsOtherErrors(t *testing.T) {
	s := &Server{cfg: &Config{Listen: "127.0.0.1:7777", Open: true}}
	if s.maybeAttach(fmt.Errorf("permission denied")) {
		t.Fatal("non-conflict error attached")
	}
}

func TestBindHintPointsAtExistingServer(t *testing.T) {
	addr := stubAddr(t, `{"version":"dev","mode":"local","authKind":"token","authenticated":false}`, http.StatusOK)
	err := bindHint(addr, addrInUseErr())
	if !isAddrInUse(err) {
		t.Fatalf("hint lost the wrapped error: %v", err)
	}
	if !strings.Contains(err.Error(), "--open") || !strings.Contains(err.Error(), addr) {
		t.Fatalf("hint missing pointer: %v", err)
	}
}

func TestBindHintPassesThroughForStrangers(t *testing.T) {
	addr := stubAddr(t, `{"ok":true}`, http.StatusOK)
	err := bindHint(addr, addrInUseErr())
	if strings.Contains(err.Error(), "--open") {
		t.Fatalf("impostor got a sift hint: %v", err)
	}
}
