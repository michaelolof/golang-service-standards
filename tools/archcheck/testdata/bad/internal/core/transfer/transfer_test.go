package transfer

type fakeNotifier struct{}

func (fakeNotifier) Notify(Transfer) error { return nil }

var _ Notifier = fakeNotifier{}
