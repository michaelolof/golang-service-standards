package a

import (
	"errors"

	"b"
)

type CancelArgs struct {
	TenantID   string `validate:"required"`
	TransferID string `lint:"required"`
	ActorID    string `json:"actor_id" validate:"required,uuid"`
	Reason     string `validate:"omitempty"`
	Kind       string `validate:"required_if=Reason x"`
	note       string `lint:"required"`
}

type Pair[T any] struct {
	Left  T `lint:"required"`
	Right T
}

type Alias = CancelArgs

type Plain struct {
	Name string `json:"name"`
}

var errFailed = errors.New("failed")

func complete() {
	_ = CancelArgs{TenantID: "t", TransferID: "x", ActorID: "a", note: "n"}
	_ = &CancelArgs{TenantID: "t", TransferID: "x", ActorID: "a", note: "n"}
	_ = CancelArgs{"t", "x", "a", "r", "k", "n"}
	_ = Plain{}
	_ = Pair[int]{Left: 1}
	_ = b.Remote{ID: "1"}
}

func incomplete() {
	_ = CancelArgs{TenantID: "t"}                                    // want `CancelArgs is missing required fields TransferID, ActorID, note`
	_ = &CancelArgs{}                                                // want `CancelArgs is missing required fields TenantID, TransferID, ActorID, note`
	_ = []CancelArgs{{TenantID: "t", TransferID: "x", ActorID: "a"}} // want `CancelArgs is missing required field note`
	_ = []*CancelArgs{{TenantID: "t", TransferID: "x", note: "n"}}   // want `CancelArgs is missing required field ActorID`
	_ = Pair[int]{Right: 1}                                          // want `Pair\[int\] is missing required field Left`
	_ = Alias{ActorID: "a"}                                          // want `Alias is missing required fields TenantID, TransferID, note`
	_ = b.Remote{}                                                   // want `b.Remote is missing required field ID`
}

func load() (CancelArgs, error) {
	return CancelArgs{}, errFailed
}

func loadPointer() (*CancelArgs, error) {
	return &CancelArgs{}, errFailed
}

func loadNil() (CancelArgs, error) {
	return CancelArgs{}, nil // want `CancelArgs is missing required fields TenantID, TransferID, ActorID, note`
}

func lookup() (CancelArgs, bool) {
	return CancelArgs{}, false
}

func lookupFound() (CancelArgs, bool) {
	return CancelArgs{}, true // want `CancelArgs is missing required fields TenantID, TransferID, ActorID, note`
}

func resolve() (CancelArgs, bool, error) {
	return CancelArgs{}, false, nil
}
