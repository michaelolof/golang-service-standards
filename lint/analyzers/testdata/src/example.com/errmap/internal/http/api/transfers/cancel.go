package transfers

import (
	"errors"

	"example.com/errmap/internal/http/httperr"
	"github.com/michaelolof/errx"
	"github.com/michaelolof/gofi"
)

func cancel() error { return errors.New("x") }

var CancelTransferHandler = gofi.DefineHandler(gofi.RouteOptions{
	Handler: func(c gofi.Context) error {
		s, err := gofi.ValidateAndBind[cancelTransferSchema](c)
		if err != nil {
			return err // want `a handler returns its errors through httperr: return httperr.Map\(stamp, err\) \(ERR-7\)`
		}

		if err := cancel(); errx.IsKind(err, "not_found") {
			return httperr.Map(0, err).WithMsg("transfer not found")
		}

		if err := cancel(); err != nil {
			return errx.Wrap(0, err) // want `a handler returns its errors through httperr`
		}

		if s == nil {
			return httperr.New(0, "BAD_REQUEST", "empty")
		}

		retry := func() error { return cancel() }
		_ = retry

		return c.Send(200, s)
	},
})

type cancelTransferSchema struct{}

var GetTransferHandler = gofi.DefineHandler(gofi.RouteOptions{Handler: get})

func get(c gofi.Context) error {
	if err := cancel(); err != nil {
		return httperr.Map(0, err)
	}

	return cancel() // want `a handler returns its errors through httperr`
}

// Middleware isn't a handler, so its returns aren't checked.
func middleware(c gofi.Context) error {
	if err := cancel(); err != nil {
		return err
	}

	return c.Next()
}

var _ = middleware

// ListTransfersHandler hands its work to a helper, whose returns are checked instead.
var ListTransfersHandler = gofi.DefineHandler(gofi.RouteOptions{
	Handler: func(c gofi.Context) error {
		list, err := ListTransfers(c, "tenant")
		if err != nil {
			return err
		}

		return c.Send(200, list)
	},
})

// ListTransfers is a handler helper: it takes the gofi.Context and other values.
func ListTransfers(c gofi.Context, tenant string) ([]string, error) {
	if tenant == "" {
		return nil, httperr.New(0, "BAD_REQUEST", "tenant required")
	}

	if err := cancel(); err != nil {
		return nil, err // want `a handler returns its errors through httperr`
	}

	if err := cancel(); err != nil {
		return nil, mapCancel(c, err)
	}

	return []string{tenant}, nil
}

func mapCancel(c gofi.Context, err error) error {
	return httperr.Map(0, err)
}

// ExportHandler wraps a helper's mapped error once more, as wrapcheck asks for a
// call into another package.
var ExportHandler = gofi.DefineHandler(gofi.RouteOptions{
	Handler: func(c gofi.Context) error {
		options, err := Options(c)
		if err != nil {
			return errx.Wrap(0, err)
		}

		if err := cancel(); err != nil {
			return errx.Wrap(0, err) // want `a handler returns its errors through httperr`
		}

		return c.Send(200, options)
	},
})

// Options takes only the context, and returns a value too, so it's a helper.
func Options(c gofi.Context) ([]string, error) {
	if err := cancel(); err != nil {
		return nil, httperr.Map(0, err)
	}

	return nil, nil
}
