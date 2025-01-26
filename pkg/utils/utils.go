package utils

import (
	"os"
	"regexp"
	"strings"
)

var (
	codeBlockRe = regexp.MustCompile(`(?s)\x60\x60\x60go(.*?)\x60\x60\x60`)
)

func SafeWriteFile(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ExtractCodeBlock(response string) string {
	matches := codeBlockRe.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return response
}
