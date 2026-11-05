package a

import (
	"gofi"

	"ext"
	"ignored"
)

type local struct{}

func (local) Save() error { return nil }

func helper() error { return nil }

func genericHelper[T any]() (*T, error) { return nil, nil }

// A return in a closure.
func closures(repo ext.Repo) {
	_ = func() error { return ext.F() }            // want `error returned from a closure should be wrapped \(ERR-3\): sig: func ext\.F\(\) error`
	_ = func() error { return repo.Save() }        // want `error returned from a closure should be wrapped`
	_ = func() (int, error) { return ext.Pair() }  // want `error returned from a closure should be wrapped`
	_ = func() (int, error) { return repo.Load() } // want `error returned from a closure should be wrapped`
	_ = func() error { return (ext.F()) }          // want `error returned from a closure should be wrapped`
	_ = func() func() error {
		return func() error { return ext.F() } // want `error returned from a closure should be wrapped`
	}
}

// A generic call, in a named function and in a closure.
func generics(c gofi.Context) error {
	s, err := gofi.ValidateAndBind[int](c)
	if err != nil {
		return err // want `error returned from a generic function should be wrapped \(ERR-3\): sig: func gofi\.ValidateAndBind\[T any\]`
	}

	_ = s

	if _, err := ext.Bind[int](); err != nil {
		return err // want `error returned from a generic function should be wrapped`
	}

	if _, err := ext.Two[int, string](); err != nil {
		return err // want `error returned from a generic function should be wrapped`
	}

	_ = func() error {
		_, err := ext.Bind[int]()
		return err // want `error returned from a generic function should be wrapped`
	}

	var declared = ext.Check[int]()
	if declared != nil {
		return declared // want `error returned from a generic function should be wrapped`
	}

	return ext.Check[int]() // want `error returned from a generic function should be wrapped`
}

func genericTuple() (*int, error) {
	return ext.Bind[int]() // want `error returned from a generic function should be wrapped`
}

// wrapcheck reports these, so a second finding would only be noise.
func wrapcheckReports(repo ext.Repo) error {
	if err := ext.F(); err != nil {
		return err
	}

	if err := repo.Save(); err != nil {
		return err
	}

	_ = func() error {
		err := ext.F()
		return err
	}

	if _, err := ext.Infer(1); err != nil {
		return err
	}

	return ext.F()
}

// gofi's own errors pass through middleware and handlers unwrapped.
func middleware(c gofi.Context) error {
	return c.Next()
}

func closureIgnored(c gofi.Context) {
	_ = func() error { return c.Next() }
	_ = func() error { return c.Send(200, nil) }
	_ = func() error { return c.SendBytes(200, nil) }
	_ = func() error { return c.SendString(200, "") }
	_ = func() error { return c.SendStream(200, nil, nil) }
	_ = func() error { return c.Close() } // want `error returned from a closure should be wrapped`
}

// Ignored by signature or by package.
func ignoredCalls(err error) {
	_ = func() error { return ext.Wrap(err) }
	_ = func() error { return ignored.F() }
	_ = func() error {
		_, err := ignored.Bind[int]()
		return err
	}
}

// Within the package, and calls that aren't to a named function.
func notExternal(fns []func() error, field struct{ fns []func() error }) {
	_ = func() error { return helper() }
	_ = func() error { return local{}.Save() }
	_ = func() error {
		_, err := genericHelper[int]()
		return err
	}
	_ = func() error { return fns[0]() }
	_ = func() error { return field.fns[0]() }
	_ = func() string { return "not an error" }
	_ = func() error { return nil }
}

func inNamedFunction() error {
	_, err := genericHelper[int]()
	return err
}

// The last assignment decides.
func reassigned(wrap func(error) error) error {
	_, err := ext.Bind[int]()
	if err != nil {
		err = wrap(err)
		return err
	}

	return nil
}
