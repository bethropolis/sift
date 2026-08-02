package logger

import (
	"strings"
	"testing"
)

type stringWriter struct{ strings.Builder }

func TestDefaultLoggerSuppressesInfo(t *testing.T) {
	var out stringWriter
	l := New(&out, false, false)
	l.Info("setup detail")
	l.Warn("action needed")
	if strings.Contains(out.String(), "setup detail") {
		t.Error("default logger emitted informational detail")
	}
	if !strings.Contains(out.String(), "action needed") {
		t.Error("default logger suppressed warning")
	}
}

func TestVerboseLoggerIncludesInfo(t *testing.T) {
	var out stringWriter
	l := New(&out, true, false)
	l.Info("diagnostic")
	if !strings.Contains(out.String(), "diagnostic") {
		t.Error("verbose logger suppressed informational detail")
	}
}
