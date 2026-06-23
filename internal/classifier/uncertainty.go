package classifier

func resolveHybrid(bm25Result, cosineResult *SimilarityResult) *SimilarityResult {
	bm25Strong := bm25Result != nil && bm25Result.Strong
	cosineStrong := cosineResult != nil && cosineResult.Strong

	if !bm25Strong && !cosineStrong {
		return nil
	}

	exemplarLevel := ""
	var source string
	agreement := false

	if bm25Strong && cosineStrong {
		if bm25Result.Level == cosineResult.Level {
			exemplarLevel = bm25Result.Level
			source = "bm25+cosine agreement"
			agreement = true
		} else {
			bm25Clear := !bm25Result.Ambiguous
			if bm25Clear && bm25Result.Confidence >= cosineResult.Confidence {
				exemplarLevel = bm25Result.Level
				source = "bm25 (stronger, margin ok)"
			} else if !cosineResult.Ambiguous && cosineResult.Confidence > bm25Result.Confidence {
				cosineLevel := cosineResult.Level
				if levelRank(cosineLevel) > levelRank("hard") {
					cosineLevel = "hard"
				}
				exemplarLevel = cosineLevel
				source = "cosine fuzzy rescue (capped at hard)"
			} else {
				exemplarLevel = maxLevel(bm25Result.Level, cosineResult.Level)
				exemplarLevel = maxLevel(exemplarLevel, "hard")
				source = "disagreement safer fallback"
			}
		}
	} else if bm25Strong {
		exemplarLevel = bm25Result.Level
		source = "bm25 only"
	} else {
		cosineLevel := cosineResult.Level
		if levelRank(cosineLevel) > levelRank("hard") {
			cosineLevel = "hard"
		}
		exemplarLevel = cosineLevel
		source = "cosine only (capped at hard)"
	}

	if exemplarLevel == "" {
		return nil
	}

	result := &SimilarityResult{
		Method:    "hybrid",
		Level:     exemplarLevel,
		Source:    source,
		Agreement: agreement,
	}

	if bm25Result != nil && cosineResult != nil {
		result.Confidence = (bm25Result.Confidence + cosineResult.Confidence) / 2.0
		if agreement {
			result.Confidence = mathMin(1.0, result.Confidence+0.15)
		}
	} else if bm25Result != nil {
		result.Confidence = bm25Result.Confidence
		result.Margin = bm25Result.Margin
	} else {
		result.Confidence = cosineResult.Confidence
		result.Margin = cosineResult.Margin
	}

	result.Strong = true
	result.LevelScores = make(map[string]float64)
	if bm25Result != nil {
		for l, s := range bm25Result.LevelScores {
			result.LevelScores[l] = s
		}
	}
	return result
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
