package config

// Scoring holds the numeric tuning for the relevance ranker and the selection
// optimizer. Every field defaults to 0, which the consuming engine interprets
// as "use the built-in default". This keeps tuning values config-driven from a
// [scoring] TOML section while letting the defaults live once, in the engine.
type Scoring struct {
	// Composite weights for the git/role/centrality relevance score.
	RecencyWeight    float64 `toml:"recency_weight"`
	ChurnWeight      float64 `toml:"churn_weight"`
	CentralityWeight float64 `toml:"centrality_weight"`
	RoleWeight       float64 `toml:"role_weight"`
	// Mode bands for the relevance score.
	FullBand float64 `toml:"full_band"`
	SkipBand float64 `toml:"skip_band"`

	// Utility optimizer knobs.
	BaseMin         float64 `toml:"base_min"`
	BaseMax         float64 `toml:"base_max"`
	RelevanceWeight float64 `toml:"relevance_weight"`
	PreferenceBonus float64 `toml:"preference_bonus"`
	SignatureBonus  float64 `toml:"signature_bonus"`
	SkipMultiplier  float64 `toml:"skip_multiplier"`
	SigQualityMin   float64 `toml:"sig_quality_min"`
	SigQualityMax   float64 `toml:"sig_quality_max"`
	TestTaskBoost   float64 `toml:"test_task_boost"`
	// AreaDiminishing controls marginal utility for additional files in one area.
	AreaDiminishing float64           `toml:"area_diminishing"`
	AreaBudgetShare float64           `toml:"area_budget_share"`
	AreaRoots       map[string]string `toml:"area_roots"`

	// Retention overrides per lang role name (e.g. "entrypoint", "docs").
	// An empty map leaves the lang-default retention in place; a present key
	// overrides it. Config takes priority over the default.
	Retention map[string]float64 `toml:"retention"`
	// SkipRoles excludes these classified file roles from automatic selection.
	// An omitted value uses the built-in test/fixture/mock/generated/vendor list.
	SkipRoles []string `toml:"skip_roles"`
}

// Merge overlays the non-zero fields of o onto s. Pointer/zero semantics mean
// a profile's explicit values win while unset fields keep s's value.
func (s *Scoring) Merge(o Scoring) {
	if o.RecencyWeight != 0 {
		s.RecencyWeight = o.RecencyWeight
	}
	if o.ChurnWeight != 0 {
		s.ChurnWeight = o.ChurnWeight
	}
	if o.CentralityWeight != 0 {
		s.CentralityWeight = o.CentralityWeight
	}
	if o.RoleWeight != 0 {
		s.RoleWeight = o.RoleWeight
	}
	if o.FullBand != 0 {
		s.FullBand = o.FullBand
	}
	if o.SkipBand != 0 {
		s.SkipBand = o.SkipBand
	}
	if o.BaseMin != 0 {
		s.BaseMin = o.BaseMin
	}
	if o.BaseMax != 0 {
		s.BaseMax = o.BaseMax
	}
	if o.RelevanceWeight != 0 {
		s.RelevanceWeight = o.RelevanceWeight
	}
	if o.PreferenceBonus != 0 {
		s.PreferenceBonus = o.PreferenceBonus
	}
	if o.SignatureBonus != 0 {
		s.SignatureBonus = o.SignatureBonus
	}
	if o.SkipMultiplier != 0 {
		s.SkipMultiplier = o.SkipMultiplier
	}
	if o.SigQualityMin != 0 {
		s.SigQualityMin = o.SigQualityMin
	}
	if o.SigQualityMax != 0 {
		s.SigQualityMax = o.SigQualityMax
	}
	if o.TestTaskBoost != 0 {
		s.TestTaskBoost = o.TestTaskBoost
	}
	if o.AreaDiminishing != 0 {
		s.AreaDiminishing = o.AreaDiminishing
	}
	if o.AreaBudgetShare != 0 {
		s.AreaBudgetShare = o.AreaBudgetShare
	}
	if len(o.AreaRoots) > 0 {
		if s.AreaRoots == nil {
			s.AreaRoots = map[string]string{}
		}
		for prefix, area := range o.AreaRoots {
			s.AreaRoots[prefix] = area
		}
	}
	if len(o.Retention) > 0 {
		if s.Retention == nil {
			s.Retention = map[string]float64{}
		}
		for k, v := range o.Retention {
			s.Retention[k] = v
		}
	}
	if o.SkipRoles != nil {
		s.SkipRoles = append([]string(nil), o.SkipRoles...)
	}
}
