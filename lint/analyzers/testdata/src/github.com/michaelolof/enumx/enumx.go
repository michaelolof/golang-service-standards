package enumx

type collection[T comparable, V any] struct{}

func (collection[T, V]) ById(id T) (T, bool) { var zero T; return zero, false }

func New[T comparable, V any](fallback *T) (group collection[T, V], setter func(k T, v V) T, getter func(val T) V) {
	return collection[T, V]{}, func(k T, v V) T { return k }, func(val T) V { var zero V; return zero }
}
