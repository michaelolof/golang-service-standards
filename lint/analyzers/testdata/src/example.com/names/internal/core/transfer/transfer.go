package transfer

import "errors"

type IStore interface{ Get() error } // want `name the interface IStore by its role, without the I prefix: -er, Store or a domain noun \(ABS-3\)`

type Store interface {
	GetTransfer() error // want `a getter is named for what it returns: Transfer, not GetTransfer \(STY-2\)`
	Get(id string) error
}

type IDGenerator interface{ New() [16]byte }

type KeyManager struct{} // want `name KeyManager by its role, not by how it's built: -er, Store, Repo or a domain noun \(ABS-3\)`

type storeImpl struct{} // want `name storeImpl by its role`

type CancelParams struct{} // want `a use case takes <Op>Args and returns <Op>Result: rename CancelParams \(ABS-6\)`

type CancelArg struct{} // want `rename CancelArg \(ABS-6\)`

type CancelArgs struct{}

type CancelResult struct{}

// Params isn't a use case's input, so it may keep its name.
type Params struct{ Memory uint32 }

type Service struct{}

func (s *Service) Cancel(args CancelParams) (CancelResult, error) { return CancelResult{}, nil }

func (s *Service) Settle(args CancelArg) error { return nil }

type Amount struct{ minor int64 }

func (a Amount) GetMinor() int64 { return a.minor } // want `a getter is named for what it returns: Minor, not GetMinor \(STY-2\)`

func (a Amount) Minor() int64 { return a.minor }

func BuildAmount(minor int64) Amount { return Amount{minor: minor} } // want `a constructor is New, or New<Type> when the package has several types: rename BuildAmount \(STY-2\)`

func NewAmount(minor int64) Amount { return Amount{minor: minor} }

func MakeLabel(s string) string { return s }

func errorNames(f func() error) error {
	e := f() // want `an error variable is err, or <verb>Err for a second one in scope: rename e \(STY-2\)`
	if e != nil {
		return e
	}

	err := f()
	rbErr := f()

	var cause error = errors.Join(err, rbErr) // want `rename cause`

	for _, pe := range []error{cause} { // want `rename pe`
		_ = pe
	}

	_ = storeImpl{}

	return nil
}

var _ = errorNames
