package pointer

func MakePtr[T any](v T) *T {
	return &v
}
