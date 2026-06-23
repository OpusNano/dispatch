package classifier

import (
	"math"
	"sort"
	"strings"

	"dispatch/internal/config"
)

type exemplarEntry struct {
	Idx       int
	Tokens    []string
	Level     string
	Negative  bool
	Weight    float64
	DocLength int
	Evidence  []string
}

type bm25Index struct {
	postings    map[string][]exemplarPosting
	idf         map[string]float64
	avgdl       float64
	k1          float64
	b           float64
	minScore    float64
	minMargin   float64
	maxPerLevel int
}

type exemplarPosting struct {
	exemplarIdx int
	tf          float64
}

type cosineIndex struct {
	vectors     []sparseVector
	buckets     int
	wordNgrams  []int
	charNgrams  []int
	minScore    float64
	minMargin   float64
	maxPerLevel int
	enabled     bool
}

type sparseVector struct {
	data map[int]float64
	norm float64
}

type hybridIndex struct {
	bm25     *bm25Index
	cosine   *cosineIndex
	resolver config.SimilarityConfig
}

type SimilarityResult struct {
	Method      string             `json:"method"`
	Level       string             `json:"suggested_level"`
	Confidence  float64            `json:"confidence"`
	Margin      float64            `json:"margin"`
	Strong      bool               `json:"strong"`
	Ambiguous   bool               `json:"ambiguous"`
	LevelScores map[string]float64 `json:"level_scores"`
	Source      string             `json:"source"`
	Agreement   bool               `json:"agreement"`
}

var exemplars []exemplarEntry
var hybrid *hybridIndex

func InitExemplars(cfg *config.Config) error {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled {
		return nil
	}
	if !cfg.Intelligence.Exemplars.Enabled {
		return nil
	}

	builtIn := loadBuiltInExemplars()
	external := []config.ExemplarDef{}
	if cfg.Intelligence.Exemplars.Path != "" {
		loaded, err := config.LoadExemplars(cfg.Intelligence.Exemplars.Path)
		if err != nil && cfg.Intelligence.Exemplars.MergeMode == "replace" {
			return err
		}
		if err == nil {
			external = loaded
		}
	}

	merged, err := config.MergeExemplars(builtIn, external, cfg.Intelligence.Exemplars.MergeMode)
	if err != nil {
		return err
	}

	exemplars = nil
	for i, e := range merged {
		tokens := tokenizeExemplar(e.Text)
		exemplars = append(exemplars, exemplarEntry{
			Idx:       i,
			Tokens:    tokens,
			Level:     e.Level,
			Negative:  e.Negative,
			Weight:    1.0,
			DocLength: len(tokens),
			Evidence:  e.EvidenceRequired,
		})
	}

	sc := cfg.Intelligence.Similarity
	hybrid = &hybridIndex{
		resolver: sc,
	}

	if sc.Mode == "hybrid" || sc.Mode == "bm25" {
		hybrid.bm25 = buildBM25(sc.BM25)
	}
	if sc.Mode == "hybrid" || sc.Mode == "cosine" {
		hybrid.cosine = buildCosine(sc.HashedCosine)
	}

	return nil
}

func loadBuiltInExemplars() []config.ExemplarDef {
	return nil
}

func tokenizeExemplar(text string) []string {
	normalized := strings.ToLower(text)
	var result []string
	var current strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	if len(result) > 1 {
		n := len(result)
		for i := 0; i < n-1; i++ {
			result = append(result, result[i]+"_"+result[i+1])
		}
	}
	return result
}

func buildBM25(cfg struct {
	Enabled              bool    `yaml:"enabled"`
	K1                   float64 `yaml:"k1"`
	B                    float64 `yaml:"b"`
	MinScore             float64 `yaml:"min_score"`
	MinMargin            float64 `yaml:"min_margin"`
	MaxExemplarsPerLevel int     `yaml:"max_exemplars_per_level"`
}) *bm25Index {
	if !cfg.Enabled || len(exemplars) == 0 {
		return nil
	}

	idx := &bm25Index{
		postings:    make(map[string][]exemplarPosting),
		idf:         make(map[string]float64),
		k1:          cfg.K1,
		b:           cfg.B,
		minScore:    cfg.MinScore,
		minMargin:   cfg.MinMargin,
		maxPerLevel: cfg.MaxExemplarsPerLevel,
	}

	totalDL := 0
	docFreq := make(map[string]int)

	for i, e := range exemplars {
		totalDL += e.DocLength
		seen := make(map[string]bool)
		tf := make(map[string]float64)
		for _, t := range e.Tokens {
			tf[t]++
			seen[t] = true
		}
		for t := range seen {
			docFreq[t]++
			idx.postings[t] = append(idx.postings[t], exemplarPosting{exemplarIdx: i, tf: tf[t]})
		}
	}

	idx.avgdl = float64(totalDL) / float64(len(exemplars))
	N := float64(len(exemplars))
	for t, df := range docFreq {
		idf := math.Log(1.0 + (N-float64(df)+0.5)/(float64(df)+0.5))
		if idf > 0 {
			idx.idf[t] = idf
		}
	}

	return idx
}

func buildCosine(cfg struct {
	Enabled    bool    `yaml:"enabled"`
	Buckets    int     `yaml:"buckets"`
	WordNgrams []int   `yaml:"word_ngrams"`
	CharNgrams []int   `yaml:"char_ngrams"`
	MinScore   float64 `yaml:"min_score"`
	MinMargin  float64 `yaml:"min_margin"`
}) *cosineIndex {
	if !cfg.Enabled || len(exemplars) == 0 {
		return nil
	}

	idx := &cosineIndex{
		buckets:     cfg.Buckets,
		wordNgrams:  cfg.WordNgrams,
		charNgrams:  cfg.CharNgrams,
		minScore:    cfg.MinScore,
		minMargin:   cfg.MinMargin,
		maxPerLevel: 5,
		enabled:     true,
	}
	if idx.buckets <= 0 {
		idx.buckets = 4096
	}

	idx.vectors = make([]sparseVector, len(exemplars))
	for i, e := range exemplars {
		vec := make(map[int]float64)
		for _, t := range e.Tokens {
			h := hashToken(t) % idx.buckets
			vec[h]++
		}
		var sumSq float64
		for _, v := range vec {
			sumSq += v * v
		}
		norm := math.Sqrt(sumSq)
		if norm > 0 {
			for h, v := range vec {
				vec[h] = v / norm
			}
		}
		idx.vectors[i] = sparseVector{data: vec, norm: norm}
	}

	return idx
}

func hashToken(t string) int {
	var h uint32 = 2166136261
	for i := 0; i < len(t); i++ {
		h ^= uint32(t[i])
		h *= 16777619
	}
	return int(h)
}

func ComputeSimilarity(text string, facts Facts) *SimilarityResult {
	if hybrid == nil || len(exemplars) == 0 {
		return nil
	}

	queryTokens := tokenizeExemplar(text)

	var bm25Result *SimilarityResult
	var cosineResult *SimilarityResult

	if hybrid.bm25 != nil {
		bm25Result = computeBM25(queryTokens, facts, text)
	}
	if hybrid.cosine != nil {
		cosineResult = computeCosine(queryTokens, facts, text)
	}

	return resolveHybrid(bm25Result, cosineResult)
}

func computeBM25(queryTokens []string, facts Facts, text string) *SimilarityResult {
	idx := hybrid.bm25
	if idx == nil {
		return nil
	}

	levelScores := make(map[string]float64)
	exemplarScores := make([]float64, len(exemplars))

	seenExemplars := make(map[int]float64)
	for _, t := range queryTokens {
		idf, ok := idx.idf[t]
		if !ok {
			continue
		}
		for _, p := range idx.postings[t] {
			e := &exemplars[p.exemplarIdx]
			score := idf * (p.tf * (idx.k1 + 1)) / (p.tf + idx.k1*(1-idx.b+idx.b*float64(e.DocLength)/idx.avgdl))
			mult := checkEvidence(e.Evidence, facts, text)
			if mult == 0 {
				continue
			}
			seenExemplars[p.exemplarIdx] += score * mult
		}
	}

	for ei, score := range seenExemplars {
		exemplarScores[ei] = score
	}

	return aggregateScores(exemplarScores, levelScores, idx.minScore, idx.minMargin, idx.maxPerLevel, "bm25")
}

func computeCosine(queryTokens []string, facts Facts, text string) *SimilarityResult {
	idx := hybrid.cosine
	if idx == nil {
		return nil
	}

	qVec := make(map[int]float64)
	for _, t := range queryTokens {
		h := hashToken(t) % idx.buckets
		qVec[h]++
	}
	var qNormSq float64
	for _, v := range qVec {
		qNormSq += v * v
	}
	qNorm := math.Sqrt(qNormSq)
	if qNorm == 0 {
		return nil
	}

	levelScores := make(map[string]float64)
	exemplarScores := make([]float64, len(exemplars))

	for ei, ev := range idx.vectors {
		var dot float64
		for h, qv := range qVec {
			if ev_val, ok := ev.data[h]; ok {
				dot += qv * ev_val
			}
		}
		cosine := dot / (qNorm * ev.norm)
		if cosine > 0 {
			mult := checkEvidence(exemplars[ei].Evidence, facts, text)
			if mult > 0 {
				exemplarScores[ei] = cosine * mult
			}
		}
	}

	return aggregateScores(exemplarScores, levelScores, idx.minScore, idx.minMargin, idx.maxPerLevel, "cosine")
}

func checkEvidence(required []string, facts Facts, text string) float64 {
	if len(required) == 0 {
		return 1.0
	}

	strength := 1.0
	for _, req := range required {
		found := false
		switch req {
		case "stack_trace":
			found = hasEvidence(facts.Evidence, EvidenceStackTrace)
		case "test_failure":
			found = hasEvidence(facts.Evidence, EvidenceTestFailure)
		case "compile_error":
			found = hasEvidence(facts.Evidence, EvidenceCompileError)
		case "tool_error":
			found = hasEvidence(facts.Evidence, EvidenceToolError)
		case "repeated_failure":
			found = hasEvidence(facts.Evidence, EvidenceRepeatedFailure)
		case "json_schema":
			found = hasEvidence(facts.Evidence, EvidenceJSONSchema)
		case "tool_calls":
			found = hasEvidence(facts.Evidence, EvidenceToolCalls)
		case "code_block":
			found = hasEvidence(facts.Evidence, EvidenceCodeBlock)
		case "logs":
			found = hasEvidence(facts.Evidence, EvidenceLogs)
		case "multi_file_scope":
			found = facts.Scope == ScopeMultiFile
		case "destructive_action":
			found = facts.DestructiveAction
		case "secret_leak_evidence":
			found = facts.SecretLeakEvidence
		case "access_impact":
			found = facts.AccessImpact
		case "transaction_impact":
			found = hasDomain(facts, DomainPayment) && rePaymentFail.MatchString(text)
		case "customer_impact":
			found = facts.CustomerImpact
		case "outage_evidence":
			found = facts.OutageEvidence
		case "production_context":
			found = facts.ProductionContext
		}

		if !found {
			switch req {
			case "destructive_action", "secret_leak_evidence", "access_impact",
				"transaction_impact", "customer_impact", "outage_evidence":
				return 0.0
			default:
				strength *= 0.3
			}
		}
	}
	return strength
}

type scoredExemplar struct {
	idx   int
	score float64
}

func aggregateScores(exemplarScores []float64, levelScores map[string]float64,
	minScore, minMargin float64, maxPerLevel int, method string) *SimilarityResult {

	var allScores []scoredExemplar
	for ei, score := range exemplarScores {
		if score > 0 {
			allScores = append(allScores, scoredExemplar{ei, score})
		}
	}
	sort.Slice(allScores, func(i, j int) bool { return allScores[i].score > allScores[j].score })

	levelBest := make(map[string]float64)
	levelCount := make(map[string]int)
	for _, s := range allScores {
		e := &exemplars[s.idx]
		if levelCount[e.Level] >= maxPerLevel {
			continue
		}
		if e.Negative {
			levelScores[e.Level] -= s.score
		} else {
			levelScores[e.Level] += s.score
			if s.score > levelBest[e.Level] {
				levelBest[e.Level] = s.score
			}
		}
		levelCount[e.Level]++
	}

	for l := range levelScores {
		if levelScores[l] < 0 {
			levelScores[l] = 0
		}
	}

	var bestLevel string
	var bestScore float64
	var totalScore float64
	for l, s := range levelScores {
		totalScore += s
		if s > bestScore {
			bestScore = s
			bestLevel = l
		}
	}

	if bestScore < minScore || bestLevel == "" || totalScore == 0 {
		return nil
	}

	confidence := bestScore / totalScore
	if confidence > 1 {
		confidence = 1
	}

	var margin float64
	if len(allScores) >= 2 {
		margin = allScores[0].score - allScores[1].score
	}

	return &SimilarityResult{
		Method:      method,
		Level:       bestLevel,
		Confidence:  confidence,
		Margin:      margin,
		Strong:      confidence >= minScore,
		Ambiguous:   margin < minMargin,
		LevelScores: levelScores,
	}
}
