// Package transfer moves money — between accounts. // want `comments use plain ASCII punctuation: replace '—', so a search finds the text \(STY-5\)`
package transfer

// Cancel follows §6.2 of the design. // want `a comment stands on its own after the plan is gone: remove the reference to "§" \(STY-5\)`
func Cancel() {}

// Settle was added in Phase 3. // want `remove the reference to "Phase 3"`
func Settle() {}

// Park is described in improvements.md. // want `remove the reference to ".md"`
func Park() {}

func Initiate() {
	// Step 1: check the balance. // want `no Step N narration: split the function into named steps instead \(STY-5\)`
	// TODO: handle retries. // want `a TODO names its ticket and the action: TODO\(TICKET-123\): <action> \(STY-5\)`
	// TODO(PAY-123): handle retries.
}

// Refund pays back Zoë, whose name has a letter beyond ASCII.
func Refund() {}

//nolint:gocognit // this directive's reason may say anything — even this
func Batch() {}
