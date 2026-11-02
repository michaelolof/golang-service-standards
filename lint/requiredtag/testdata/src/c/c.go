package c

import "errors"

type Args struct {
	ID string `lint:"required"`
}

func load() (Args, error) {
	return Args{}, errors.New("failed") // want `Args is missing required field ID`
}
