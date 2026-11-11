package jobs

type Kind string

type Job struct {
	Kind Kind
	Body []byte
}

const (
	KindTransferInitiate Kind = "transfer.initiate"
	KindEraseActor       Kind = "erasure.erase_actor"
	KindLegacy           Kind = "ProcessTransfer" // want `a job kind is <capability>.<action>, such as "transfer.initiate": "ProcessTransfer" \(JOB-1\)`
)

// MaxAttempts isn't a kind.
const MaxAttempts = 5
