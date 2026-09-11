package radius

import "strings"

// IsAccountingLog reports whether a RouterOS syslog line is relevant RADIUS
// accounting activity. Syslog is intentionally treated only as an untrusted
// wake-up signal; packet attributes are never reconstructed or applied to
// identity state. Authoritative state is always re-read from RouterOS API.
func IsAccountingLog(raw string) bool {
	line := strings.ToLower(raw)
	if !strings.Contains(line, "radius,debug,packet") {
		return false
	}
	return strings.Contains(line, "accounting-request") ||
		strings.Contains(line, "accounting-response") ||
		strings.Contains(line, "acct-status-type")
}
