package ports

import "example.com/layers/internal/domain"

type Publisher interface{ Publish(domain.Transfer) error }

type PublishArgs struct{ Transfer domain.Transfer }

func NewPublisher() Publisher { return nil } // want `ports holds contracts only: NewPublisher belongs with an implementation \(LAY-6\)`

func newArgs() PublishArgs { return PublishArgs{} }

var _ = newArgs
