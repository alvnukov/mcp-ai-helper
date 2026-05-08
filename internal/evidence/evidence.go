// Package evidence extracts and validates compact evidence references.
package evidence

import (
	"regexp"
	"strings"
)

// Line is one numbered evidence item.
type Line struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Text   string `json:"text"`
}

// Summary contains evidence extracted from a larger artifact.
type Summary struct {
	EvidenceLines []Line `json:"evidence_lines"`
	Truncated     bool   `json:"truncated"`
}

// Validation reports whether analysis is grounded in known evidence ids.
type Validation struct {
	Valid      bool     `json:"valid"`
	Status     string   `json:"status"`
	References []string `json:"references"`
	Problems   []string `json:"problems,omitempty"`
}

var evidenceRefPattern = regexp.MustCompile(`\[(E[0-9]+)\]|\b(E[0-9]+)\b`)

// Select returns a bounded set of relevant evidence lines.
func Select(lines []string, limit int) []Line {
	if limit <= 0 {
		limit = 30
	}
	keywords := []string{"error", "failed", "failure", "panic", "fatal", "exception", "denied", "timeout", "traceback", "undefined", "cannot", "no such file"}
	selected := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		matched := false
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		selected = append(selected, trimmed)
		if len(selected) >= limit {
			break
		}
	}
	if len(selected) == 0 {
		for _, line := range tail(lines, limit) {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				selected = append(selected, trimmed)
			}
		}
	}
	out := make([]Line, 0, len(selected))
	for i, line := range selected {
		out = append(out, Line{ID: "E" + itoa(i+1), Source: "command_output", Text: line})
	}
	return out
}

// ValidateLinks checks that analysis only cites evidence ids present in summary.
func ValidateLinks(summary Summary, analysis string, requireEvidence bool) Validation {
	known := make(map[string]struct{}, len(summary.EvidenceLines))
	for _, line := range summary.EvidenceLines {
		known[line.ID] = struct{}{}
	}
	matches := evidenceRefPattern.FindAllStringSubmatch(analysis, -1)
	refs := make([]string, 0, len(matches))
	problems := []string{}
	seen := map[string]struct{}{}
	for _, match := range matches {
		ref := match[1]
		if ref == "" {
			ref = match[2]
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
		if _, ok := known[ref]; !ok {
			problems = append(problems, "unknown evidence reference "+ref)
		}
	}
	if requireEvidence && strings.TrimSpace(analysis) != "" && len(refs) == 0 {
		problems = append(problems, "analysis has no evidence references")
	}
	valid := len(problems) == 0
	status := "ok"
	if !valid {
		status = "INSUFFICIENT_DATA"
	}
	return Validation{Valid: valid, Status: status, References: refs, Problems: problems}
}

func tail(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
