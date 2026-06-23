package classifier

import (
	"math/rand/v2"
	"strings"
	"testing"

	"dispatch/internal/config"
)

func TestClassifyNoPanicOnWeirdContent(t *testing.T) {
	cfg := loadConfig(t)

	weirdInputs := []Input{
		{Messages: nil},
		{Messages: []Message{}},
		{Messages: []Message{{Role: "", Content: ""}}},
		{Messages: []Message{{Role: "user", Content: strings.Repeat("A", 1_000_000)}}},
		{Messages: []Message{{Role: "user", Content: "\x00\x01\x02\x03\x04"}}},
		{Messages: []Message{{Role: "user", Content: "🎉🎊 unicode emoji"}}},
		{Messages: []Message{{Role: "user", Content: "中文测试 日本語テスト"}}},
		{Messages: []Message{{Role: "user", Content: "\n\r\t\v\f"}}},
		{Messages: []Message{{Role: "user", Content: `{"nested":"json","as":"string"}`}}},
		{Messages: []Message{{Role: "user", Content: "<script>alert('xss')</script>"}}},
		{Messages: []Message{{Role: "user", Content: "production database migration rollback security CVE"}}},
		{HasTools: true, HasResponseFormat: true},
	}

	for i, input := range weirdInputs {
		result := ClassifySimple(input, cfg)
		validLevels := map[string]bool{"easy": true, "medium": true, "hard": true, "critical": true}
		if !validLevels[result.Level] {
			t.Errorf("weird input %d: invalid level %q", i, result.Level)
		}
	}
}

func TestClassifyRandomGarbage(t *testing.T) {
	cfg := loadConfig(t)
	rng := rand.New(rand.NewPCG(42, 1024))

	charset := "abcdefghijklmnopqrstuvwxyz0123456789 \n\t.,;:!?{}[]()<>=+-*/&|%^#@~"
	for i := 0; i < 500; i++ {
		length := rng.IntN(500)
		buf := make([]byte, length)
		for j := range buf {
			buf[j] = charset[rng.IntN(len(charset))]
		}
		input := Input{
			Messages: []Message{{Role: "user", Content: string(buf)}},
		}
		result := ClassifySimple(input, cfg)
		validLevels := map[string]bool{"easy": true, "medium": true, "hard": true, "critical": true}
		if !validLevels[result.Level] {
			t.Errorf("random input %d: invalid level %q", i, result.Level)
		}
	}
}

func FuzzClassify(f *testing.F) {
	f.Add("hello world")
	f.Add("production database migration rollback")
	f.Add("")
	f.Add("{{{{}}}}")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, content string) {
		cfg, err := config.DefaultConfig()
		if err != nil {
			t.Skip()
		}
		input := Input{
			Messages: []Message{{Role: "user", Content: content}},
		}
		result := ClassifySimple(input, cfg)
		validLevels := map[string]bool{"easy": true, "medium": true, "hard": true, "critical": true}
		if !validLevels[result.Level] {
			t.Errorf("fuzz: invalid level %q for content %q", result.Level, content)
		}
	})
}

func TestClassifyLargeMessageCount(t *testing.T) {
	cfg := loadConfig(t)
	msgs := make([]Message, 1000)
	for i := range msgs {
		msgs[i] = Message{Role: "user", Content: "test message"}
	}
	result := ClassifySimple(Input{Messages: msgs}, cfg)
	if result.Level == "" {
		t.Error("should return valid level for large message count")
	}
}
