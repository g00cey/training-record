package llmeval

import "strings"

// Validation bounds. llm/server.py already validates its own output, but
// the backend never trusts the wire twice.
const (
	maxSummaryRunes  = 1000
	maxListItems     = 10
	maxListItemRunes = 200
	maxRawRunes      = 8000
)

// Normalize trims empty/whitespace-only entries and caps string lengths and
// list sizes in the LLM's output.
func Normalize(res *Result) *Result {
	return &Result{
		Summary:     capRunes(strings.TrimSpace(res.Summary), maxSummaryRunes),
		Strengths:   capList(res.Strengths),
		Concerns:    capList(res.Concerns),
		Suggestions: capList(res.Suggestions),
		Raw:         capRunes(res.Raw, maxRawRunes),
	}
}

func capList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		out = append(out, capRunes(it, maxListItemRunes))
		if len(out) >= maxListItems {
			break
		}
	}
	return out
}

func capRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
