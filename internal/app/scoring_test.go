package app

import (
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
)

func TestWeightsFromScoringEmptyUsesDefaults(t *testing.T) {
	w := WeightsFromScoring(config.Scoring{})
	if w != rank.DefaultWeights() {
		t.Fatalf("empty section must equal engine defaults, got %+v", w)
	}
}

func TestWeightsFromScoringOverrides(t *testing.T) {
	w := WeightsFromScoring(config.Scoring{RecencyWeight: 0.9, RoleWeight: 0.3})
	def := rank.DefaultWeights()
	if w.ChurnWeight != def.ChurnWeight {
		t.Errorf("ChurnWeight = %v, want default %v", w.ChurnWeight, def.ChurnWeight)
	}
	if w.RecencyWeight != 0.9 {
		t.Errorf("RecencyWeight = %v, want 0.9", w.RecencyWeight)
	}
	if w.RoleWeight != 0.3 {
		t.Errorf("RoleWeight = %v, want 0.3", w.RoleWeight)
	}
}

func TestTuningFromScoringRetentionCopied(t *testing.T) {
	tun := TuningFromScoring(config.Scoring{
		BaseMin:   0.02,
		Retention: map[string]float64{"entrypoint": 0.8},
	})
	if tun.BaseMin != 0.02 {
		t.Errorf("BaseMin = %v, want 0.02", tun.BaseMin)
	}
	def := selection.DefaultTuning()
	if tun.BaseMax != def.BaseMax {
		t.Errorf("BaseMax = %v, want default %v", tun.BaseMax, def.BaseMax)
	}
	if tun.Retention["entrypoint"] != 0.8 {
		t.Errorf("retention override not copied, got %v", tun.Retention)
	}
}
