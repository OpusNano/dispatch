package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFileSimpleKeyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("OPENROUTER_API_KEY=sk-or-v1-test-key\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-test-key" {
		t.Errorf("expected sk-or-v1-test-key, got %q", val)
	}
}

func TestParseEnvFileKeyNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("OTHER_VAR=value\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}

func TestParseEnvFileCommentsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("# this is a comment\nOPENROUTER_API_KEY=sk-or-v1-from-comment-block\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-from-comment-block" {
		t.Errorf("expected sk-or-v1-from-comment-block, got %q", val)
	}
}

func TestParseEnvFileBlankLinesIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("\n\n\nOPENROUTER_API_KEY=sk-or-v1-after-blanks\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-after-blanks" {
		t.Errorf("expected sk-or-v1-after-blanks, got %q", val)
	}
}

func TestParseEnvFileWhitespaceTrimmed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("  OPENROUTER_API_KEY  =  sk-or-v1-padded  \n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-padded" {
		t.Errorf("expected sk-or-v1-padded, got %q", val)
	}
}

func TestParseEnvFileExportSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("export OPENROUTER_API_KEY=sk-or-v1-exported\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-exported" {
		t.Errorf("expected sk-or-v1-exported, got %q", val)
	}
}

func TestParseEnvFileDoubleQuoted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("OPENROUTER_API_KEY=\"sk-or-v1-quoted\"\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-quoted" {
		t.Errorf("expected sk-or-v1-quoted, got %q", val)
	}
}

func TestParseEnvFileSingleQuoted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("OPENROUTER_API_KEY='sk-or-v1-single-quoted'\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "sk-or-v1-single-quoted" {
		t.Errorf("expected sk-or-v1-single-quoted, got %q", val)
	}
}

func TestParseEnvFileMissingFile(t *testing.T) {
	_, err := ParseEnvFile("/nonexistent/.env_does_not_exist", "OPENROUTER_API_KEY")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseEnvFileReturnsFirstMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("OPENROUTER_API_KEY=first\nOPENROUTER_API_KEY=second\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "first" {
		t.Errorf("expected first, got %q", val)
	}
}

func TestParseEnvFileEmptyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("OPENROUTER_API_KEY=\n"), 0644)
	val, err := ParseEnvFile(path, "OPENROUTER_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}
