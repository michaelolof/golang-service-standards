// Package errx is a stand-in for the real library.
package errx

type errKind string

func Kind(k string) errKind                          { return errKind(k) }
func IsKind(err error, kind errKind) bool            { return false }
func NewKind(ts int, kind errKind, msg string) error { return nil }
func WrapKind(ts int, kind errKind, err error) error { return err }
