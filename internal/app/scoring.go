package app

import (
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
)

// WeightsFromScoring maps a config [scoring] section onto the ranker's
// composite weights. Zero config fields fall back to the engine defaults so
// an empty section behaves identically to not configuring anything.
func WeightsFromScoring(s config.Scoring) rank.Weights {
	def := rank.DefaultWeights()
	w := def
	if s.RecencyWeight != 0 {
		w.RecencyWeight = s.RecencyWeight
	}
	if s.ChurnWeight != 0 {
		w.ChurnWeight = s.ChurnWeight
	}
	if s.CentralityWeight != 0 {
		w.CentralityWeight = s.CentralityWeight
	}
	if s.RoleWeight != 0 {
		w.RoleWeight = s.RoleWeight
	}
	if s.FullBand != 0 {
		w.FullBand = s.FullBand
	}
	if s.SkipBand != 0 {
		w.SkipBand = s.SkipBand
	}
	return w
}

// TuningFromScoring converts a config [scoring] section onto the selection
// optimizer's tuning. Zero config fields fall back to the engine defaults;
// the Retention map is copied through verbatim so config can override the
// lang-default retention for any role.
func TuningFromScoring(s config.Scoring) selection.Tuning {
	def := selection.DefaultTuning()
	t := def
	if s.BaseMin != 0 {
		t.BaseMin = s.BaseMin
	}
	if s.BaseMax != 0 {
		t.BaseMax = s.BaseMax
	}
	if s.RelevanceWeight != 0 {
		t.RelevanceWeight = s.RelevanceWeight
	}
	if s.PreferenceBonus != 0 {
		t.PreferenceBonus = s.PreferenceBonus
	}
	if s.SignatureBonus != 0 {
		t.SignatureBonus = s.SignatureBonus
	}
	if s.SkipMultiplier != 0 {
		t.SkipMultiplier = s.SkipMultiplier
	}
	if s.SigQualityMin != 0 {
		t.SigQualityMin = s.SigQualityMin
	}
	if s.SigQualityMax != 0 {
		t.SigQualityMax = s.SigQualityMax
	}
	if s.TestTaskBoost != 0 {
		t.TestTaskBoost = s.TestTaskBoost
	}
	if s.AreaDiminishing != 0 {
		t.AreaDiminishing = s.AreaDiminishing
	}
	if s.AreaBudgetShare != 0 {
		t.AreaBudgetShare = s.AreaBudgetShare
	}
	if s.AreaRoots != nil {
		t.AreaRoots = make(map[string]string, len(s.AreaRoots))
		for prefix, area := range s.AreaRoots {
			t.AreaRoots[prefix] = area
		}
	}
	if s.Retention != nil {
		t.Retention = make(map[string]float64, len(s.Retention))
		for k, v := range s.Retention {
			t.Retention[k] = v
		}
	}
	if s.SkipRoles != nil {
		t.SkipRoles = append([]string(nil), s.SkipRoles...)
	}
	return t
}
