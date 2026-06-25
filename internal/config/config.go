package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

type OpenRouterConfig struct {
	BaseURL               string `yaml:"base_url"`
	APIKeyEnv             string `yaml:"api_key_env"`
	APIKeyFile            string `yaml:"api_key_file"`
	ValidateModelsOnStart bool   `yaml:"validate_models_on_start"`
	HTTPReferer           string `yaml:"http_referer"`
	SiteTitle             string `yaml:"site_title"`
}

type ServerConfig struct {
	Listen              string `yaml:"listen"`
	MaxBodySize         int64  `yaml:"max_body_size"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

type ProviderConfig struct {
	Order          []string `yaml:"order"`
	Only           []string `yaml:"only,omitempty"`
	Ignore         []string `yaml:"ignore,omitempty"`
	AllowFallbacks *bool    `yaml:"allow_fallbacks"`
	DataCollection string   `yaml:"data_collection,omitempty"`
}

type ModelProfile struct {
	Id       string         `yaml:"id"`
	Provider ProviderConfig `yaml:"provider,omitempty"`
}

type LevelConfig struct {
	Use      string         `yaml:"use,omitempty"`
	Model    string         `yaml:"model,omitempty"`
	Provider ProviderConfig `yaml:"provider,omitempty"`
}

type ResolvedModel struct {
	Model       string
	Provider    ProviderConfig
	Source      string // "profile" or "inline"
	ProfileName string // set when Source == "profile"
}

type ThresholdsConfig struct {
	Easy                          float64 `yaml:"easy"`
	EasyMax                       float64 `yaml:"easy_max"`
	MediumMax                     float64 `yaml:"medium_max"`
	HardMax                       float64 `yaml:"hard_max"`
	RiskCriticalOverride          float64 `yaml:"risk_critical_override"`
	RiskHardFloor                 float64 `yaml:"risk_hard_floor"`
	AgentPressureCriticalOverride float64 `yaml:"agent_pressure_critical_override"`
}

type ScoringWeights struct {
	Complexity    float64 `yaml:"complexity"`
	Risk          float64 `yaml:"risk"`
	AgentPressure float64 `yaml:"agent_pressure"`
	Downgrade     float64 `yaml:"downgrade"`
}

type ScoringConfig struct {
	ComplexityCap    float64        `yaml:"complexity_cap"`
	RiskCap          float64        `yaml:"risk_cap"`
	AgentPressureCap float64        `yaml:"agent_pressure_cap"`
	DowngradeCap     float64        `yaml:"downgrade_cap"`
	Weights          ScoringWeights `yaml:"weights"`
}

type PatternRule struct {
	ID                  string   `yaml:"id"`
	Regex               string   `yaml:"regex,omitempty"`
	Dimension           string   `yaml:"dimension"`
	Weight              float64  `yaml:"weight"`
	Reason              string   `yaml:"reason"`
	MatchTools          bool     `yaml:"match_tools,omitempty"`
	MatchResponseFormat bool     `yaml:"match_response_format,omitempty"`
	Requires            []string `yaml:"requires,omitempty"`
	RequiresNot         []string `yaml:"requires_not,omitempty"`
	PerMatch            bool     `yaml:"per_match,omitempty"`
	Cap                 float64  `yaml:"cap,omitempty"`
}

type DebugConfig struct {
	LogLevel            string `yaml:"log_level"`
	LogPrompts          bool   `yaml:"log_prompts"`
	LogMetadata         bool   `yaml:"log_metadata"`
	LogDecisions        bool   `yaml:"log_decisions"`
	TraceRequests       bool   `yaml:"trace_requests"`
	SetResponseHeaders  bool   `yaml:"set_response_headers"`
	RequestIndexEnabled bool   `yaml:"request_index_enabled"`
	RequestIndexSize    int    `yaml:"request_index_size"`
	FeedbackEnabled     bool   `yaml:"feedback_enabled"`
	FeedbackPath        string `yaml:"feedback_path"`
}

type ConfigReloadConfig struct {
	Enabled             bool `yaml:"enabled"`
	PollIntervalSeconds int  `yaml:"poll_interval_seconds"`
}

type LengthConfig struct {
	MaxLevelWithoutEvidence string `yaml:"max_level_without_evidence"`
}

type SessionEscalationConfig struct {
	HardAfterRepeatedFailures     int `yaml:"hard_after_repeated_failures"`
	CriticalAfterRepeatedFailures int `yaml:"critical_after_repeated_failures"`
	DecayPerSuccess               int `yaml:"decay_per_success"`
}

type SessionConfig struct {
	Enabled              bool                    `yaml:"enabled"`
	RequireSessionHeader bool                    `yaml:"require_session_header"`
	Header               string                  `yaml:"header"`
	FallbackKey          string                  `yaml:"fallback_key"`
	TTLMinutes           int                     `yaml:"ttl_minutes"`
	MaxEntries           int                     `yaml:"max_entries"`
	Escalation           SessionEscalationConfig `yaml:"escalation"`
}

type ProfileDef struct {
	MinMargin           float64 `yaml:"min_margin"`
	ExemplarFloorWeight float64 `yaml:"exemplar_floor_weight"`
	FloorReduction      int     `yaml:"floor_reduction"`
	ForceMinLevel       string  `yaml:"force_min_level"`
}

type RoutingProfilesConfig struct {
	Default             string                `yaml:"default"`
	AllowHeaderOverride bool                  `yaml:"allow_header_override"`
	Header              string                `yaml:"header"`
	Profiles            map[string]ProfileDef `yaml:"profiles"`
}

type ConceptDef struct {
	Name    string   `yaml:"name"`
	Aliases []string `yaml:"aliases"`
}

type OpObjRuleDef struct {
	Operation        string   `yaml:"operation"`
	Object           string   `yaml:"object"`
	MinLevel         string   `yaml:"min_level"`
	Reason           string   `yaml:"reason"`
	RequiresEvidence []string `yaml:"requires_evidence,omitempty"`
}

type OpObjConfig struct {
	ProximityWindow int            `yaml:"proximity_window"`
	Rules           []OpObjRuleDef `yaml:"rules"`
}

type ExemplarDef struct {
	Text             string   `yaml:"text"`
	Level            string   `yaml:"level"`
	Task             string   `yaml:"task,omitempty"`
	Operation        string   `yaml:"operation,omitempty"`
	Object           string   `yaml:"object,omitempty"`
	Role             string   `yaml:"role,omitempty"`
	EvidenceRequired []string `yaml:"evidence_required,omitempty"`
	Negative         bool     `yaml:"negative,omitempty"`
}

type ExemplarsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Path      string `yaml:"path"`
	MergeMode string `yaml:"merge_mode"`
}

type SimilarityConfig struct {
	Mode string `yaml:"mode"`
	BM25 struct {
		Enabled              bool    `yaml:"enabled"`
		K1                   float64 `yaml:"k1"`
		B                    float64 `yaml:"b"`
		MinScore             float64 `yaml:"min_score"`
		MinMargin            float64 `yaml:"min_margin"`
		MaxExemplarsPerLevel int     `yaml:"max_exemplars_per_level"`
	} `yaml:"bm25"`
	HashedCosine struct {
		Enabled    bool    `yaml:"enabled"`
		Buckets    int     `yaml:"buckets"`
		WordNgrams []int   `yaml:"word_ngrams"`
		CharNgrams []int   `yaml:"char_ngrams"`
		MinScore   float64 `yaml:"min_score"`
		MinMargin  float64 `yaml:"min_margin"`
	} `yaml:"hashed_cosine"`
	Resolver struct {
		AgreementBonus            float64 `yaml:"agreement_bonus"`
		DisagreementFallbackLevel string  `yaml:"disagreement_fallback_level"`
		WeakSimilarityFallback    string  `yaml:"weak_similarity_fallback"`
		CosineOnlyMaxLevel        string  `yaml:"cosine_only_max_level"`
	} `yaml:"resolver"`
}

type UncertaintyConfig struct {
	SaferFallback       bool    `yaml:"safer_fallback"`
	ExemplarFloorWeight float64 `yaml:"exemplar_floor_weight"`
}

type IntelligenceConfig struct {
	Enabled         bool                  `yaml:"enabled"`
	Length          LengthConfig          `yaml:"length"`
	Concepts        []ConceptDef          `yaml:"concepts"`
	OperationObject OpObjConfig           `yaml:"operation_object"`
	Exemplars       ExemplarsConfig       `yaml:"exemplars"`
	Similarity      SimilarityConfig      `yaml:"similarity"`
	Uncertainty     UncertaintyConfig     `yaml:"uncertainty"`
	Session         SessionConfig         `yaml:"session"`
	RoutingProfiles RoutingProfilesConfig `yaml:"routing_profiles"`
}

type Config struct {
	OpenRouter    OpenRouterConfig        `yaml:"openrouter"`
	Server        ServerConfig            `yaml:"server"`
	ModelProfiles map[string]ModelProfile `yaml:"model_profiles"`
	Levels        map[string]LevelConfig  `yaml:"levels"`
	Thresholds    ThresholdsConfig        `yaml:"thresholds"`
	Scoring       ScoringConfig           `yaml:"scoring"`
	Patterns      []PatternRule           `yaml:"patterns"`
	Debug         DebugConfig             `yaml:"debug"`
	ConfigReload  ConfigReloadConfig      `yaml:"config_reload"`
	Intelligence  *IntelligenceConfig     `yaml:"intelligence,omitempty"`
	Version       string                  `yaml:"version"`

	compiledPatterns map[string]*compiledRule
}

type compiledRule struct {
	Pattern PatternRule
	Regex   *regexp.Regexp
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if err := cfg.CompileAndValidate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) CompileAndValidate() error {
	if c.Server.MaxBodySize <= 0 {
		c.Server.MaxBodySize = 26214400
	}
	if c.Debug.RequestIndexSize <= 0 {
		c.Debug.RequestIndexSize = 500
	}
	if c.ConfigReload.PollIntervalSeconds <= 0 {
		c.ConfigReload.PollIntervalSeconds = 3
	}
	if err := c.validate(); err != nil {
		return err
	}
	compiled, err := c.compilePatterns()
	if err != nil {
		return err
	}
	c.compiledPatterns = compiled
	return nil
}

func (c *Config) ResolveLevel(level string) (ResolvedModel, bool) {
	lc, ok := c.Levels[level]
	if !ok {
		return ResolvedModel{}, false
	}

	if lc.Model != "" {
		return ResolvedModel{
			Model:    lc.Model,
			Provider: lc.Provider,
			Source:   "inline",
		}, true
	}

	if lc.Use != "" {
		mp, ok := c.ModelProfiles[lc.Use]
		if !ok {
			return ResolvedModel{}, false
		}
		return ResolvedModel{
			Model:       mp.Id,
			Provider:    mp.Provider,
			Source:      "profile",
			ProfileName: lc.Use,
		}, true
	}

	return ResolvedModel{}, false
}

func (c *Config) validate() error {
	errs := make([]string, 0)

	if c.OpenRouter.BaseURL == "" {
		errs = append(errs, "openrouter.base_url is required")
	}
	if c.OpenRouter.APIKeyEnv == "" {
		errs = append(errs, "openrouter.api_key_env is required")
	}

	if len(c.ModelProfiles) == 0 {
		errs = append(errs, "model_profiles: at least one profile is required")
	}
	for name, mp := range c.ModelProfiles {
		if mp.Id == "" {
			errs = append(errs, fmt.Sprintf("model_profiles.%s.id is required", name))
		}
	}

	validLevels := map[string]bool{"easy": true, "medium": true, "hard": true, "critical": true}
	for _, level := range []string{"easy", "medium", "hard", "critical"} {
		lc, ok := c.Levels[level]
		if !ok {
			errs = append(errs, fmt.Sprintf("levels.%s is required", level))
			continue
		}
		if lc.Use == "" && lc.Model == "" {
			errs = append(errs, fmt.Sprintf("levels.%s: must set either 'use' or 'model'", level))
		}
		if lc.Use != "" && lc.Model != "" {
			errs = append(errs, fmt.Sprintf("levels.%s: cannot set both 'use' and 'model'", level))
		}
		if lc.Use != "" {
			if _, ok := c.ModelProfiles[lc.Use]; !ok {
				errs = append(errs, fmt.Sprintf("levels.%s: 'use' references unknown profile %q", level, lc.Use))
			}
		}
		if lc.Model != "" && lc.Use == "" {
			// inline model — valid as long as model is non-empty
			if lc.Provider.Order == nil && lc.Provider.AllowFallbacks == nil && lc.Provider.DataCollection == "" &&
				len(lc.Provider.Only) == 0 && len(lc.Provider.Ignore) == 0 {
				// empty provider is fine (defaults apply)
			}
		}
	}
	for k := range c.Levels {
		if !validLevels[k] {
			errs = append(errs, fmt.Sprintf("levels.%s: unknown level (must be easy|medium|hard|critical)", k))
		}
	}

	if c.Thresholds.EasyMax <= c.Thresholds.Easy {
		errs = append(errs, "thresholds.easy_max > thresholds.easy is required")
	}
	if c.Thresholds.MediumMax <= c.Thresholds.EasyMax {
		errs = append(errs, "thresholds.medium_max > easy_max is required")
	}
	if c.Thresholds.HardMax <= c.Thresholds.MediumMax {
		errs = append(errs, "thresholds.hard_max > medium_max is required")
	}

	if c.Scoring.ComplexityCap < 0 || c.Scoring.RiskCap < 0 || c.Scoring.AgentPressureCap < 0 || c.Scoring.DowngradeCap < 0 {
		errs = append(errs, "all scoring caps must be >= 0")
	}

	seenIDs := map[string]bool{}
	for i, p := range c.Patterns {
		if p.ID == "" {
			errs = append(errs, fmt.Sprintf("patterns[%d].id is required", i))
			continue
		}
		if seenIDs[p.ID] {
			errs = append(errs, fmt.Sprintf("patterns: duplicate id %q", p.ID))
		}
		seenIDs[p.ID] = true

		if p.Dimension != "complexity" && p.Dimension != "risk" && p.Dimension != "agent_pressure" && p.Dimension != "downgrade" {
			errs = append(errs, fmt.Sprintf("pattern %q: dimension must be complexity|risk|agent_pressure|downgrade", p.ID))
		}
		if !p.MatchTools && !p.MatchResponseFormat && p.Regex == "" && len(p.Requires) == 0 {
			errs = append(errs, fmt.Sprintf("pattern %q: must specify regex, match_tools, match_response_format, or requires", p.ID))
		}
		if p.Regex != "" {
			if _, err := regexp.Compile("(?i)" + p.Regex); err != nil {
				errs = append(errs, fmt.Sprintf("pattern %q: invalid regex: %v", p.ID, err))
			}
		}
	}

	for _, p := range c.Patterns {
		for _, ref := range p.Requires {
			if !seenIDs[ref] {
				errs = append(errs, fmt.Sprintf("pattern %q: requires references unknown pattern %q", p.ID, ref))
			}
		}
		for _, ref := range p.RequiresNot {
			if !seenIDs[ref] {
				errs = append(errs, fmt.Sprintf("pattern %q: requires_not references unknown pattern %q", p.ID, ref))
			}
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

func (c *Config) compilePatterns() (map[string]*compiledRule, error) {
	compiled := make(map[string]*compiledRule, len(c.Patterns))
	for i := range c.Patterns {
		p := c.Patterns[i]
		cr := &compiledRule{Pattern: p}
		if p.Regex != "" {
			re, err := regexp.Compile("(?i)" + p.Regex)
			if err != nil {
				return nil, fmt.Errorf("pattern %q: compile %q: %w", p.ID, p.Regex, err)
			}
			cr.Regex = re
		}
		compiled[p.ID] = cr
	}
	return compiled, nil
}

func (c *Config) CompiledPatterns() map[string]*compiledRule {
	return c.compiledPatterns
}

func (c *Config) CompiledPatternsOrdered() []*compiledRule {
	result := make([]*compiledRule, 0, len(c.Patterns))
	for i := range c.Patterns {
		if cr, ok := c.compiledPatterns[c.Patterns[i].ID]; ok {
			result = append(result, cr)
		}
	}
	return result
}

func (cr *compiledRule) MatchesText(text string) bool {
	if cr.Regex == nil {
		return false
	}
	return cr.Regex.MatchString(text)
}

func (cr *compiledRule) FindAllMatches(text string) [][]int {
	if cr.Regex == nil {
		return nil
	}
	return cr.Regex.FindAllStringIndex(text, -1)
}

type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	s := "config validation failed:"
	for _, msg := range e.Errors {
		s += "\n  - " + msg
	}
	return s
}

func EnsureConfigDir(configDir string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("cannot create config dir %s: %w\n\nEnsure %s is a writable bind-mount with correct owner.\n  For example: chown 65532:65532 ./config\n  or use docker run -v ./config:/config with proper permissions.\n", configDir, err, configDir)
	}
	yamlPath := filepath.Join(configDir, "router.yaml")
	mdPath := filepath.Join(configDir, "DISPATCH.md")
	exemplarsPath := filepath.Join(configDir, "exemplars.yaml")

	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		if err := os.WriteFile(yamlPath, []byte(defaultConfigYAML), 0644); err != nil {
			return fmt.Errorf("cannot write %s: %w\n\nEnsure %s is a writable bind-mount with correct owner.\n", yamlPath, err, configDir)
		}
	}
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		if err := os.WriteFile(mdPath, []byte(defaultROUTERmd), 0644); err != nil {
			return fmt.Errorf("cannot write %s: %w\n\nEnsure %s is a writable bind-mount with correct owner.\n", mdPath, err, configDir)
		}
	}
	if _, err := os.Stat(exemplarsPath); os.IsNotExist(err) {
		if err := os.WriteFile(exemplarsPath, []byte(defaultExemplarsYAML), 0644); err != nil {
			return fmt.Errorf("cannot write %s: %w\n\nEnsure %s is a writable bind-mount with correct owner.\n", exemplarsPath, err, configDir)
		}
	}
	return nil
}

func (c *Config) PrintConfig(w io.Writer) {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	_ = enc.Encode(c)
}

func DefaultConfig() (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(defaultConfigYAML), cfg); err != nil {
		return nil, err
	}
	if err := cfg.CompileAndValidate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
