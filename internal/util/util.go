package util

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9-]+`)

func NormalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func NormalizeMAC(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.NewReplacer("-", "", ":", "", ".", "").Replace(s)
	if len(s) != 12 {
		return strings.ToUpper(strings.TrimSpace(s))
	}
	parts := make([]string, 0, 6)
	for i := 0; i < 12; i += 2 {
		parts = append(parts, s[i:i+2])
	}
	return strings.Join(parts, ":")
}

func StableClientID(username string) string {
	canonical := NormalizeUsername(username)
	slug := nonSlug.ReplaceAllString(canonical, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "user"
	}
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("radius-%s-%x", slug, sum[:4])
}

func IsIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		var n int
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
			n = n*10 + int(r-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}
