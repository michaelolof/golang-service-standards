package metric

type InstrumentOption interface{ instrument() }

type unitOption string

func (unitOption) instrument() {}

func WithUnit(unit string) InstrumentOption { return unitOption(unit) }

type Int64Counter interface{ Add(n int64) }

type Float64Histogram interface{ Record(v float64) }

type Int64Histogram interface{ Record(v int64) }

type Meter interface {
	Int64Counter(name string, opts ...InstrumentOption) (Int64Counter, error)
	Int64Histogram(name string, opts ...InstrumentOption) (Int64Histogram, error)
	Float64Histogram(name string, opts ...InstrumentOption) (Float64Histogram, error)
}
