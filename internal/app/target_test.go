package app

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		spec         string
		path, symbol string
		start, end   int
	}{
		{"main.go", "main.go", "", 0, 0},
		{"main.go#Run", "main.go", "Run", 0, 0},
		{"internal/app/app.go#App.Config", "internal/app/app.go", "App.Config", 0, 0},
		{"internal/app/app.go:10-25", "internal/app/app.go", "", 10, 25},
		{`C:\repo\main.go`, `C:\repo\main.go`, "", 0, 0},
		{`C:\repo\main.go#Run`, `C:\repo\main.go`, "Run", 0, 0},
		{`C:\repo\main.go:3-8`, `C:\repo\main.go`, "", 3, 8},
	}
	for _, tc := range cases {
		got, err := ParseTarget(tc.spec)
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", tc.spec, err)
			continue
		}
		if got.Path != tc.path || got.Symbol != tc.symbol || got.Start != tc.start || got.End != tc.end {
			t.Errorf("ParseTarget(%q) = %+v, want %+v", tc.spec, got, tc)
		}
	}

	for _, bad := range []string{"", "#Sym", "a.go#", "a.go:0-3", "a.go:9-2"} {
		if _, err := ParseTarget(bad); err == nil {
			t.Errorf("ParseTarget(%q) succeeded, want error", bad)
		}
	}
}
