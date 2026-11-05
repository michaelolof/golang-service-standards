package ext

import "errors"

type Repo interface {
	Save() error
	Load() (int, error)
}

func F() error { return errors.New("f") }

func Pair() (int, error) { return 0, errors.New("pair") }

func Bind[T any]() (*T, error) { return nil, errors.New("bind") }

func Check[T any]() error { return errors.New("check") }

func Two[K comparable, V any]() (map[K]V, error) { return nil, errors.New("two") }

func Infer[T any](v T) (T, error) { return v, errors.New("infer") }

func Wrap(err error) error { return err }
