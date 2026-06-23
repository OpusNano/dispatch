package classifier

type ContextMeta struct {
	Chars        int    `json:"chars"`
	Messages     int    `json:"messages"`
	SizeBucket   string `json:"size"`
	LengthPolicy string `json:"length_policy"`
}

func ComputeContextMetadata(combinedText string, msgCount int) ContextMeta {
	chars := len(combinedText)
	size := "small"
	if chars >= 50000 {
		size = "huge"
	} else if chars >= 10000 {
		size = "large"
	} else if chars >= 2000 {
		size = "medium"
	}
	return ContextMeta{
		Chars:        chars,
		Messages:     msgCount,
		SizeBucket:   size,
		LengthPolicy: "metadata_only",
	}
}

func applyLengthCap(level string, evidenceFloorsFired bool, gatesFired bool) string {
	if gatesFired {
		return level
	}
	if evidenceFloorsFired {
		return level
	}
	if levelRank(level) > levelRank("medium") {
		return "medium"
	}
	return level
}
