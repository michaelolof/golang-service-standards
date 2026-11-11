package payout

import "errors"

type Deps struct{ Clock func() int64 }

type Service struct{ clock func() int64 }

func New(d Deps) (*Service, error) {
	if d.Clock == nil {
		return nil, errors.New("payout.New: missing clock")
	}

	return &Service{clock: d.Clock}, nil
}
