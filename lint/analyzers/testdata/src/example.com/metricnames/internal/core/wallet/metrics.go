package wallet

import "go.opentelemetry.io/otel/metric"

func NewMetrics(m metric.Meter, name string) error {
	if _, err := m.Int64Counter("myservice.wallet.provision.attempts", metric.WithUnit("{attempt}")); err != nil {
		return err
	}

	if _, err := m.Float64Histogram("myservice.wallet.provision.duration", metric.WithUnit("s")); err != nil {
		return err
	}

	if _, err := m.Float64Histogram("edge_request_duration_ms", metric.WithUnit("ms")); err != nil { // want `an instrument is named <svc>.<capability>.<noun>, lowercase and dotted: "edge_request_duration_ms" \(OBS-2\)`
		return err
	}

	if _, err := m.Int64Counter("myservice.wallet.requests_total", metric.WithUnit("{request}")); err != nil { // want `"myservice.wallet.requests_total" has a unit suffix: set the unit with metric.WithUnit, and the exporter adds the suffix \(OBS-2\)`
		return err
	}

	if _, err := m.Int64Counter("myservice.wallet.retries"); err != nil { // want `set "myservice.wallet.retries"'s unit with metric.WithUnit, in UCUM: s, By, \{job\} \(OBS-2\)`
		return err
	}

	if _, err := m.Int64Histogram("myservice.wallet.call.duration", metric.WithUnit("ms")); err != nil { // want `a duration is a Float64Histogram measured in seconds, with metric.WithUnit\("s"\) \(OBS-2\)`
		return err
	}

	_, err := m.Int64Counter(name, metric.WithUnit("{x}")) // want `an instrument's name is a constant, <svc>.<capability>.<noun> \(OBS-2\)`

	return err
}
