package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ParseEnvFile(path, keyName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}

		k := strings.TrimSpace(line[:idx])
		if k != keyName {
			continue
		}

		v := strings.TrimSpace(line[idx+1:])
		v = stripQuotes(v)
		return v, nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan: %w", err)
	}
	return "", nil
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
