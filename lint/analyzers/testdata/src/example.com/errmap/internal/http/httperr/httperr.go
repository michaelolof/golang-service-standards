package httperr

type httperr struct{ err error }

func (e *httperr) Error() string                   { return "" }
func (e *httperr) WithMsg(msg string) *httperr     { return e }
func Map(ts int, err error) *httperr               { return &httperr{err: err} }
func New(ts int, kind string, msg string) *httperr { return &httperr{} }
