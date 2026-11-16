package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseBudget parses human-readable token budget strings such as "200k",
// "1.5M", "120k", "200_000", "200,000", or "200000". Suffixes 'k'/'K' (1,000)
// and 'm'/'M' (1,000,000) are case-insensitive and support decimal values.
func ParseBudget(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Remove internal visual separators (underscores and commas).
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, ",", "")

	multiplier := 1.0
	lower := strings.ToLower(s)
	switch {
	case strings.HasSuffix(lower, "k"):
		multiplier = 1e3
		s = s[:len(s)-1]
	case strings.HasSuffix(lower, "m"):
		multiplier = 1e6
		s = s[:len(s)-1]
	case strings.HasSuffix(lower, "g"):
		multiplier = 1e9
		s = s[:len(s)-1]
	}

	s = strings.TrimSpace(s)
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: invalid syntax", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("budget must be non-negative: %v", val)
	}

	total := val * multiplier
	return int(total), nil
}

// parseBudgetAny converts a TOML-decoded budget value (which may be an int64,
// float64, or string like "200k") into an integer.
func parseBudgetAny(v any) (int, error) {
	if v == nil {
		return 0, nil
	}
	switch val := v.(type) {
	case int64:
		if val < 0 {
			return 0, fmt.Errorf("budget must be non-negative: %d", val)
		}
		return int(val), nil
	case int:
		if val < 0 {
			return 0, fmt.Errorf("budget must be non-negative: %d", val)
		}
		return val, nil
	case float64:
		if val < 0 {
			return 0, fmt.Errorf("budget must be non-negative: %v", val)
		}
		return int(val), nil
	case string:
		return ParseBudget(val)
	default:
		return 0, fmt.Errorf("unexpected budget value type %T: %v", v, v)
	}
}

// budgetFlag binds an integer pointer to a pflag.Value, accepting both raw
// numbers and human-friendly unit strings ("200k", "1.5M").
type budgetFlag struct {
	val *int
}

func (b *budgetFlag) String() string {
	if b.val == nil {
		return "0"
	}
	return strconv.Itoa(*b.val)
}

func (b *budgetFlag) Set(s string) error {
	v, err := ParseBudget(s)
	if err != nil {
		return err
	}
	*b.val = v
	return nil
}

func (b *budgetFlag) Type() string {
	return "budget"
}

func (s *SiftDefaults) normalize() error {
	if s == nil {
		return nil
	}
	if s.BudgetRaw != nil {
		b, err := parseBudgetAny(s.BudgetRaw)
		if err != nil {
			return err
		}
		s.Budget = b
	}
	return nil
}

func (t *Target) normalize() error {
	if t == nil {
		return nil
	}
	if t.BudgetRaw != nil {
		b, err := parseBudgetAny(t.BudgetRaw)
		if err != nil {
			return err
		}
		t.Budget = b
	}
	return nil
}

func (p *Profile) normalize() error {
	if p == nil {
		return nil
	}
	if p.BudgetRaw != nil {
		b, err := parseBudgetAny(p.BudgetRaw)
		if err != nil {
			return err
		}
		p.Budget = b
	}
	return nil
}

func (cf *configFile) normalize() error {
	if cf.Sift != nil {
		if err := cf.Sift.normalize(); err != nil {
			return fmt.Errorf("sift.budget: %w", err)
		}
	}
	for name, p := range cf.Profiles {
		if err := p.normalize(); err != nil {
			return fmt.Errorf("profile %q budget: %w", name, err)
		}
		cf.Profiles[name] = p
	}
	for i := range cf.Targets {
		if err := cf.Targets[i].normalize(); err != nil {
			return fmt.Errorf("target %q budget: %w", cf.Targets[i].Name, err)
		}
	}
	return nil
}

func (pf *profilesFile) normalize() error {
	for name, p := range pf.Profiles {
		if err := p.normalize(); err != nil {
			return fmt.Errorf("profile %q budget: %w", name, err)
		}
		pf.Profiles[name] = p
	}
	return nil
}
