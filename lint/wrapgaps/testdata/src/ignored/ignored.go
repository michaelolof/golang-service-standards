package ignored

import "errors"

func F() error { return errors.New("f") }

func Bind[T any]() (*T, error) { return nil, errors.New("bind") }
