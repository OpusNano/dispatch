package classifier

import (
	"regexp"
	"strings"

	"dispatch/internal/config"
)

type compiledConcept struct {
	Name  string
	Regex *regexp.Regexp
}

var compiledConcepts []compiledConcept

func InitConcepts(concepts []config.ConceptDef) {
	compiledConcepts = nil
	for _, c := range concepts {
		aliases := make([]string, len(c.Aliases)+1)
		aliases[0] = regexp.QuoteMeta(c.Name)
		for i, a := range c.Aliases {
			aliases[i+1] = regexp.QuoteMeta(a)
		}
		pattern := `(?i)\b(` + strings.Join(aliases, "|") + `)\b`
		compiledConcepts = append(compiledConcepts, compiledConcept{
			Name:  c.Name,
			Regex: regexp.MustCompile(pattern),
		})
	}
}

func ExtractConcepts(text string, cfg *config.Config) []string {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled {
		return nil
	}
	if len(compiledConcepts) == 0 && len(cfg.Intelligence.Concepts) > 0 {
		InitConcepts(cfg.Intelligence.Concepts)
	}
	var results []string
	for _, cc := range compiledConcepts {
		if cc.Regex.MatchString(text) {
			results = append(results, cc.Name)
		}
	}
	return results
}
