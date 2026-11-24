package utils

func NilIfEmpty(val string) *string {
	if val == "" {
		return nil
	}
	return &val
}
