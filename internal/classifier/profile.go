package classifier

import (
	"dispatch/internal/config"
)

type ProfileInfo struct {
	Name           string  `json:"name"`
	Source         string  `json:"source"`
	FloorReduction int     `json:"floor_reduction"`
	ForceMinLevel  string  `json:"force_min_level"`
	MinMargin      float64 `json:"min_margin"`
}

func ApplyProfile(level string, facts Facts, gatesFired bool, profileName string, cfg *config.Config) (string, *ProfileInfo) {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled {
		return level, nil
	}

	rp := cfg.Intelligence.RoutingProfiles
	if rp.Default == "" {
		return level, nil
	}
	if profileName == "" {
		profileName = rp.Default
	}

	prof, ok := rp.Profiles[profileName]
	if !ok {
		return level, nil
	}

	info := &ProfileInfo{
		Name:           profileName,
		Source:         "config",
		FloorReduction: prof.FloorReduction,
		ForceMinLevel:  prof.ForceMinLevel,
		MinMargin:      prof.MinMargin,
	}

	if prof.ForceMinLevel != "" {
		level = maxLevel(level, prof.ForceMinLevel)
	}

	if prof.FloorReduction > 0 && !gatesFired {
		hasRisk := false
		for _, d := range facts.Domains {
			if d == DomainAuth || d == DomainPayment || d == DomainSecurity || d == DomainSecrets || d == DomainDatabase || d == DomainDeployment {
				hasRisk = true
				break
			}
		}
		if facts.Risk == RiskCritical || facts.Risk == RiskHigh {
			hasRisk = true
		}

		if !hasRisk {
			for i := 0; i < prof.FloorReduction; i++ {
				reduced := reduceLevel(level)
				if prof.ForceMinLevel != "" && levelRank(reduced) < levelRank(prof.ForceMinLevel) {
					break
				}
				if reduced == level {
					break
				}
				level = reduced
			}
		}
	}

	return level, info
}

func GetProfile(headerProfile string, cfg *config.Config) (string, error) {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled {
		return "", nil
	}
	rp := cfg.Intelligence.RoutingProfiles
	if rp.Default == "" {
		return "", nil
	}

	profileName := rp.Default

	if rp.AllowHeaderOverride && headerProfile != "" {
		if _, ok := rp.Profiles[headerProfile]; !ok {
			return "", &ProfileError{
				Name:     headerProfile,
				Profiles: validProfileNames(rp.Profiles),
			}
		}
		profileName = headerProfile
	}

	return profileName, nil
}

func validProfileNames(profiles map[string]config.ProfileDef) []string {
	var names []string
	for k := range profiles {
		names = append(names, k)
	}
	return names
}

type ProfileError struct {
	Name     string
	Profiles []string
}

func (e *ProfileError) Error() string {
	return "unknown routing profile: " + e.Name
}

var _ error = (*ProfileError)(nil)
