package domain

import "strings"

// zoneNames are the heart-rate zone labels used in the spin `notes`
// convention (see domain.md). Order matches training_db.py.
var zoneNames = []string{"ウォームアップ", "インテンシブ", "有酸素", "無酸素", "最大酸素摂取量"}

// ParseHeartRateZones extracts zone -> "MM:SS" pairs from a spin session's
// free-text notes, replicating the parser in training_db.py's
// weekly-summary (take up to 8 runes after the label, cut at '/', keep it
// only if it contains ':').
func ParseHeartRateZones(notes string) map[string]string {
	const marker = "心拍ゾーン内訳:"
	i := strings.Index(notes, marker)
	if i < 0 {
		return map[string]string{}
	}
	zoneText := notes[i+len(marker):]
	out := map[string]string{}
	for _, z := range zoneNames {
		j := strings.Index(zoneText, z)
		if j < 0 {
			continue
		}
		rest := []rune(zoneText[j+len(z):])
		if len(rest) > 8 {
			rest = rest[:8]
		}
		timeStr := strings.TrimSpace(string(rest))
		if k := strings.Index(timeStr, "/"); k >= 0 {
			timeStr = timeStr[:k]
		}
		timeStr = strings.TrimSpace(timeStr)
		if timeStr != "" && strings.Contains(timeStr, ":") {
			out[z] = timeStr
		}
	}
	return out
}
