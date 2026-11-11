package domain

import (
	"strings"

	"github.com/michaelolof/enumx"
	"github.com/michaelolof/errx"
)

type TransferStatus string // want `TransferStatus's wire values are UPPER_SNAKE: "on-hold" \(DOM-6\)`

var TransferStatuses, setTransferStatus, _ = enumx.New[TransferStatus, any](nil)

var (
	TransferStatusPending = setTransferStatus("PENDING", nil)
	TransferStatusFailed  = setTransferStatus("FAILED", nil)
	PendingRefund         = setTransferStatus("PENDING_REFUND", nil) // want `an enum variable is <Type><Variant>, such as TransferStatusPending: rename PendingRefund \(DOM-6\)`
	TransferStatusOnHold  = setTransferStatus("on-hold", nil)
)

func ParseTransferStatus(s string) (TransferStatus, error) {
	if v, ok := TransferStatuses.ById(TransferStatus(s)); ok {
		return v, nil
	}

	return "", errx.New(0, "invalid transfer status")
}

func (s TransferStatus) Validate() error { _, err := ParseTransferStatus(string(s)); return err }
func (s TransferStatus) Oneof() string   { return strings.Join(nil, " ") }
