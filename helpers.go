package main

import (
	"fmt"
	"strings"
	"time"
)

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m <= 1 {
			return "1 m ago"
		}
		return fmt.Sprintf("%d m ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h <= 1 {
			return "1 h ago"
		}
		return fmt.Sprintf("%d h ago", h)
	case d < 48*time.Hour:
		return "yesterday"
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d d ago", days)
	}
}

func domainParts(name string) (string, string) {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return name, ""
	}
	return name[:i], name[i+1:]
}

func isImportantTLD(tld string) bool {
	switch tld {
	case "mil", "gov", "eu":
		return true
	}
	return false
}
