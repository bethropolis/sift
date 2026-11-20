package config

import (
	"math"
	"testing"

	"github.com/spf13/pflag"
)

func TestParseBudget(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"200000", 200000, false},
		{"200_000", 200000, false},
		{"200,000", 200000, false},
		{"200k", 200000, false},
		{"200K", 200000, false},
		{"120k", 120000, false},
		{"1.5M", 1500000, false},
		{"1.5m", 1500000, false},
		{"2.5k", 2500, false},
		{"1M", 1000000, false},
		{"  50k  ", 50000, false},
		{"invalid", 0, true},
		{"-100", 0, true},
		{"-50k", 0, true},
		{"NaN", 0, true},
		{"Inf", 0, true},
		{"-Inf", 0, true},
		{"9e300", 0, true},
	}

	for _, tc := range tests {
		got, err := ParseBudget(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseBudget(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseBudget(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseBudgetAny(t *testing.T) {
	tests := []struct {
		input   any
		want    int
		wantErr bool
	}{
		{nil, 0, false},
		{int(10000), 10000, false},
		{int64(200000), 200000, false},
		{float64(50000), 50000, false},
		{"200k", 200000, false},
		{"1.5M", 1500000, false},
		{-100, 0, true},
		{int64(-1), 0, true},
		{true, 0, true},
		{math.NaN(), 0, true},
		{math.Inf(1), 0, true},
		{float64(1e300), 0, true},
	}

	for _, tc := range tests {
		got, err := parseBudgetAny(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseBudgetAny(%v) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("parseBudgetAny(%v) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestBudgetFlag(t *testing.T) {
	var budget int
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&budgetFlag{val: &budget}, "budget", "Token budget")

	if err := fs.Parse([]string{"--budget", "200k"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if budget != 200000 {
		t.Errorf("budget = %d, want 200000", budget)
	}

	if err := fs.Parse([]string{"--budget", "1.5M"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if budget != 1500000 {
		t.Errorf("budget = %d, want 1500000", budget)
	}

	if err := fs.Parse([]string{"--budget", "invalid"}); err == nil {
		t.Errorf("expected error for invalid budget, got nil")
	}
}

func TestTOMLHumanReadableBudget(t *testing.T) {
	tomlData := []byte(`
[sift]
budget = "200k"

[profiles.mobile]
budget = "1.5M"

[[targets]]
name = "audit"
budget = "120k"

[[targets]]
name = "underscore"
budget = 200_000
`)

	cf, err := parseConfigBytes(tomlData)
	if err != nil {
		t.Fatalf("parseConfigBytes failed: %v", err)
	}

	if cf.Sift.Budget != 200000 {
		t.Errorf("Sift.Budget = %d, want 200000", cf.Sift.Budget)
	}
	if cf.Profiles["mobile"].Budget != 1500000 {
		t.Errorf("Profiles[mobile].Budget = %d, want 1500000", cf.Profiles["mobile"].Budget)
	}
	if len(cf.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(cf.Targets))
	}
	if cf.Targets[0].Budget != 120000 {
		t.Errorf("Targets[0].Budget = %d, want 120000", cf.Targets[0].Budget)
	}
	if cf.Targets[1].Budget != 200000 {
		t.Errorf("Targets[1].Budget = %d, want 200000", cf.Targets[1].Budget)
	}
}
