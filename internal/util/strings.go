package util

// NullStr maps an empty string to NULL so create payloads store a clear
// empty value rather than an empty string.
func NullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NullPtr maps a pointer to an empty string to NULL, so clients can send an
// empty string to mean "clear this optional id field".
func NullPtr(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}
