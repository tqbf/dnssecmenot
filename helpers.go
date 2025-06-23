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

var classColors = map[string]string{
	"Technology":      "bg-blue-100 text-blue-600",
	"Asia Technology": "bg-pink-100 text-pink-600",
	"Finance":         "bg-green-100 text-green-600",
	"Government":      "bg-red-100 text-red-600",
	"Manufacturing":   "bg-yellow-100 text-yellow-600",
	"Media":           "bg-purple-100 text-purple-600",
	"NGO":             "bg-teal-100 text-teal-600",
	"Retail":          "bg-orange-100 text-orange-600",
	"Telecom":         "bg-indigo-100 text-indigo-600",
}

func classColor(class string) string {
	c, ok := classColors[class]
	if !ok {
		return "bg-gray-100 text-gray-500"
	}
	return c
}
