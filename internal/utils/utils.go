package utils

// DefaultIfZero returns fallback when value is the zero value.
func DefaultIfZero[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}

// PtrIfNonZero returns nil when the value reports itself as zero.
func PtrIfNonZero[T interface{ IsZero() bool }](v T) *T {
	if v.IsZero() {
		return nil
	}

	return &v
}
