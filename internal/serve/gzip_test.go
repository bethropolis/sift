package serve

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func gzipRequest(path, accept string) *http.Request {
	r := httptest.NewRequest("GET", path, nil)
	if accept != "" {
		r.Header.Set("Accept-Encoding", accept)
	}
	return r
}

func serveGzip(t *testing.T, path, accept, contentType string, body []byte, status int) *httptest.ResponseRecorder {
	t.Helper()
	h := (&Server{}).gzipAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		if len(body) > 0 {
			_, _ = w.Write(body)
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, gzipRequest(path, accept))
	return rec
}

func TestGzipCompressesLargeJSON(t *testing.T) {
	body := []byte(`{"document":"` + strings.Repeat("x", 2048) + `"}`)
	rec := serveGzip(t, "/api/pack", "gzip", "application/json", body, http.StatusOK)
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatal("missing Vary: Accept-Encoding")
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Fatal("Content-Length was not dropped")
	}
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatal("decompressed body differs")
	}
}

func TestGzipSkipsSmallAndUnrequested(t *testing.T) {
	small := []byte(`{"ok":true}`)
	for _, tc := range []struct {
		name   string
		accept string
		body   []byte
	}{
		{"small", "gzip", small},
		{"unrequested", "", []byte(strings.Repeat("x", 2048))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveGzip(t, "/api/pack", tc.accept, "application/json", tc.body, http.StatusOK)
			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want none", got)
			}
			if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
				t.Fatal("missing Vary even when uncompressed")
			}
			if !bytes.Equal(rec.Body.Bytes(), tc.body) {
				t.Fatal("plain body differs")
			}
		})
	}
}

func TestGzipSkipsStreamingAndNoContent(t *testing.T) {
	for _, path := range []string{"/api/app/heartbeat", "/api/clone"} {
		rec := serveGzip(t, path, "gzip", "application/x-ndjson", []byte(strings.Repeat("{}\n", 2048)), http.StatusOK)
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("%s Content-Encoding = %q", path, got)
		}
	}
	rec := serveGzip(t, "/api/logout", "gzip", "application/json", nil, http.StatusNoContent)
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("204 Content-Encoding = %q", got)
	}
}

func TestGzipHonorsExistingEncoding(t *testing.T) {
	h := (&Server{}).gzipAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "br")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, gzipRequest("/api/pack", "gzip"))
	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
}
