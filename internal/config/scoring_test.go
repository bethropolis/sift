package config

import "testing"

func TestScoringMergeOverlaysNonZero(t *testing.T) {
	base := Scoring{RecencyWeight: 0.5, RoleWeight: 0.8}
	over := Scoring{RecencyWeight: 0.9, ChurnWeight: 0.3}
	base.Merge(over)
	if base.RecencyWeight != 0.9 {
		t.Errorf("RecencyWeight = %v, want 0.9 (config overrides)", base.RecencyWeight)
	}
	if base.RoleWeight != 0.8 {
		t.Errorf("RoleWeight = %v, want 0.8 (unset keeps default)", base.RoleWeight)
	}
	if base.ChurnWeight != 0.3 {
		t.Errorf("ChurnWeight = %v, want 0.3 (new field added)", base.ChurnWeight)
	}
}

func TestScoringMergeRetentionOverrides(t *testing.T) {
	base := Scoring{Retention: map[string]float64{"docs": 0.1}}
	over := Scoring{Retention: map[string]float64{"docs": 0.9, "config": 0.5}}
	base.Merge(over)
	if base.Retention["docs"] != 0.9 {
		t.Errorf("docs retention = %v, want 0.9 (config priority)", base.Retention["docs"])
	}
	if base.Retention["config"] != 0.5 {
		t.Errorf("config retention = %v, want 0.5", base.Retention["config"])
	}
}
