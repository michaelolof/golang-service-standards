package errx

type lint int

type errKind string

func Kind(k string) errKind                                    { return errKind(k) }
func IsKind(err error, kind errKind) bool                      { return false }
func New(ts lint, msg string) error                            { return nil }
func NewKind(ts lint, kind errKind, msg string) error          { return nil }
func Wrap(ts lint, err error) error                            { return err }
func WrapKind(ts lint, kind errKind, err error) error          { return err }
func NewKindf(ts lint, kind errKind, f string, a ...any) error { return nil }
