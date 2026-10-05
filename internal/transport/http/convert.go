package httpserver

func enumPtr[T ~string](value *string) *T {
	if value == nil {
		return nil
	}
	out := T(*value)
	return &out
}
