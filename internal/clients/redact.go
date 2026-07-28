package clients

import "unicode/utf8"

// redactBody truncates provider response/request bodies for logs (no full dumps in prod debug).
func redactBody(b []byte, max int) string {
	if max <= 0 {
		max = 256
	}
	s := string(b)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…[redacted]"
}
