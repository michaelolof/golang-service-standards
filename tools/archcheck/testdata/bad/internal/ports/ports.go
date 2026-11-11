// Package ports holds the contracts several core packages share.
package ports

// Mailer sends mail.
type Mailer interface{ Send() error }

// Unshared has an implementation, and nothing outside ports uses it.
type Unshared interface{ Share() }
