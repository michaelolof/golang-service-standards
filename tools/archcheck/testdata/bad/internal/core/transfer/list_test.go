package transfer_test

import "example.com/bad/internal/core/transfer"

type fakeLister struct{}

func (fakeLister) List() ([]transfer.Transfer, error) { return nil, nil }

var _ transfer.Lister = fakeLister{}
