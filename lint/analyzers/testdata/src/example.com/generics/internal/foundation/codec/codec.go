package codec

type Receipt struct{}

func kindOf[T any](value T) string {
	switch any(value).(type) { // want `don't type-switch on a type parameter: pass the behaviour that varies in as a value \(ABS-8\)`
	case Receipt:
		return "receipt"
	}

	return "other"
}

func kindOfZero[T any]() string {
	switch v := any(*new(T)).(type) { // want `don't type-switch on a type parameter`
	case Receipt:
		_ = v
		return "receipt"
	}

	return "other"
}

// Encode doesn't depend on T, so generics are right here.
func Encode[T any](value T, encode func(T) string) string { return encode(value) }

func describe(value any) string {
	switch value.(type) {
	case Receipt:
		return "receipt"
	}

	return "other"
}

var _, _, _ = kindOf[Receipt], kindOfZero[Receipt], describe
