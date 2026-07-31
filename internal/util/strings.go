package util

func NullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func StrPtr(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}
