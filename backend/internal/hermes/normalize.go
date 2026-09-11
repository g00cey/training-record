package hermes

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// CanonicalZones are the canonical heart-rate zone labels. They must match
// the frontend HR_ZONES (frontend/lib/domain.ts) and the spin notes
// convention (docs/domain.md). Order matches the form display order.
var CanonicalZones = []string{
	"ウォームアップ",
	"インテンシブ",
	"有酸素",
	"無酸素",
	"最大酸素摂取量(高負荷)",
}

// ValidUncertainFields are the field names allowed in uncertainFields.
var ValidUncertainFields = []string{
	"durationMinutes",
	"avgHeartRate",
	"maxHeartRate",
	"distanceKm",
	"hrZones",
	"freeNotes",
}

// zoneAliases maps common label variants (lower-cased, half-width parens,
// trimmed) to canonical zone names. Hermes is asked to normalise labels
// itself; this is defensive.
var zoneAliases = map[string]string{
	// canonical / suffix variants
	"最大酸素摂取量":      "最大酸素摂取量(高負荷)",
	"最大酸素摂取量(高負荷)": "最大酸素摂取量(高負荷)",
	"最大酸素摂取量(高強度)": "最大酸素摂取量(高負荷)",
	// z1..z5 / zone n（アプリのゾーン番号表記。順序は慣習フォーマットと同じ）
	"z1":     "ウォームアップ",
	"zone 1": "ウォームアップ",
	"zone1":  "ウォームアップ",
	"z2":     "インテンシブ",
	"zone 2": "インテンシブ",
	"zone2":  "インテンシブ",
	"z3":     "有酸素",
	"zone 3": "有酸素",
	"zone3":  "有酸素",
	"z4":     "無酸素",
	"zone 4": "無酸素",
	"zone4":  "無酸素",
	"z5":     "最大酸素摂取量(高負荷)",
	"zone 5": "最大酸素摂取量(高負荷)",
	"zone5":  "最大酸素摂取量(高負荷)",
	// 英語表記のつづれ対応
	"warm up":   "ウォームアップ",
	"warmup":    "ウォームアップ",
	"warm-up":   "ウォームアップ",
	"intensive": "インテンシブ",
	"aerobic":   "有酸素",
	"anaerobic": "無酸素",
	"vo2max":    "最大酸素摂取量(高負荷)",
	"vo2 max":   "最大酸素摂取量(高負荷)",
}

var mmssRe = regexp.MustCompile(`^\d{1,3}:[0-5]\d$`)

// Validation bounds. Out-of-range values are dropped (nil) and reported in
// UncertainFields instead of being trusted or clamped silently.
const (
	minDurationMinutes = 1
	maxDurationMinutes = 1440
	minHeartRate       = 60
	maxHeartRate       = 250
	maxDistanceKm      = 500.0
	maxFreeNotesRunes  = 500
	maxUncertainItems  = 10
)

// Normalize validates and canonicalises a parsed extraction result:
//   - zone labels are mapped to CanonicalZones (unknown labels are dropped)
//   - zone times must be mm:ss (mmm:ss over an hour)
//   - numeric fields are range-checked; invalid ones become nil
//   - avg > max (both present) keeps both values but flags both as uncertain
//   - freeNotes is trimmed and capped
//   - uncertainFields is filtered to known names, de-duplicated, sorted
func Normalize(res *Result) *Result {
	out := &Result{HrZones: map[string]string{}}
	uncertain := map[string]bool{}

	for _, f := range res.UncertainFields {
		f = strings.TrimSpace(f)
		for _, valid := range ValidUncertainFields {
			if f == valid {
				uncertain[f] = true
			}
		}
	}

	if res.DurationMinutes != nil {
		if *res.DurationMinutes >= minDurationMinutes && *res.DurationMinutes <= maxDurationMinutes {
			out.DurationMinutes = res.DurationMinutes
		} else {
			uncertain["durationMinutes"] = true
		}
	}
	if res.AvgHeartRate != nil {
		if *res.AvgHeartRate >= minHeartRate && *res.AvgHeartRate <= maxHeartRate {
			out.AvgHeartRate = res.AvgHeartRate
		} else {
			uncertain["avgHeartRate"] = true
		}
	}
	if res.MaxHeartRate != nil {
		if *res.MaxHeartRate >= minHeartRate && *res.MaxHeartRate <= maxHeartRate {
			out.MaxHeartRate = res.MaxHeartRate
		} else {
			uncertain["maxHeartRate"] = true
		}
	}
	if out.AvgHeartRate != nil && out.MaxHeartRate != nil && *out.AvgHeartRate > *out.MaxHeartRate {
		// 明らかな読み取り逆転。値は残すが要注意として明示する
		uncertain["avgHeartRate"] = true
		uncertain["maxHeartRate"] = true
	}
	if res.DistanceKm != nil {
		if *res.DistanceKm > 0 && *res.DistanceKm <= maxDistanceKm {
			out.DistanceKm = res.DistanceKm
		} else {
			uncertain["distanceKm"] = true
		}
	}

	for label, value := range res.HrZones {
		zone, ok := normalizeZoneKey(label)
		if !ok {
			continue
		}
		v := strings.TrimSpace(value)
		if !mmssRe.MatchString(v) {
			uncertain["hrZones"] = true
			continue
		}
		out.HrZones[zone] = v
	}

	notes := strings.TrimSpace(res.FreeNotes)
	if utf8.RuneCountInString(notes) > maxFreeNotesRunes {
		runes := []rune(notes)
		notes = string(runes[:maxFreeNotesRunes])
	}
	out.FreeNotes = notes

	fields := make([]string, 0, len(uncertain))
	for f := range uncertain {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	if len(fields) > maxUncertainItems {
		fields = fields[:maxUncertainItems]
	}
	out.UncertainFields = fields
	return out
}

// normalizeZoneKey maps a zone label to its canonical name.
func normalizeZoneKey(k string) (string, bool) {
	k = strings.TrimSpace(k)
	k = strings.ReplaceAll(k, "（", "(")
	k = strings.ReplaceAll(k, "）", ")")
	for _, z := range CanonicalZones {
		if k == z {
			return z, true
		}
	}
	if z, ok := zoneAliases[strings.ToLower(k)]; ok {
		return z, true
	}
	return "", false
}
