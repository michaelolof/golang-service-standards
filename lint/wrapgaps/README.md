# wrapgaps

A golangci-lint module plugin that reports the unwrapped errors wrapcheck can't see (ERR-3). wrapcheck v2.13.0 skips two forms:

| How the error is returned | Named function | Closure |
|---|---|---|
| `return pkg.F()` | wrapcheck | **wrapgaps** |
| `err := pkg.F()`, then `return err` | wrapcheck | wrapcheck |
| `return pkg.F[T]()` | **wrapgaps** | **wrapgaps** |
| `err := pkg.F[T]()`, then `return err` | **wrapgaps** | **wrapgaps** |

Why wrapcheck skips them:
- **A return in a closure.** It's deliberate. `wrapcheck.go` stops as soon as it finds a function literal above a direct call.
- **A generic call.** It only reads a call written `pkg.F()`. `pkg.F[T]()` parses as an index expression, so it returns without reporting. A generic call whose types are inferred is written `pkg.F()`, so wrapcheck sees it.

wrapgaps doesn't report what wrapcheck reports, so an error never gets two findings.

## What needs wrapping

The same as wrapcheck: a call to another package or to an exported interface method, unless its signature or package is on the ignore list. wrapcheck's default ignore list always applies, because wrapgaps reads it from wrapcheck. A call within the package doesn't need wrapping.

## Configuration

The settings mean what wrapcheck's settings of the same name mean. The shared config uses YAML anchors, so the two linters can't disagree:

```yaml
linters:
  enable: [wrapcheck, wrapgaps]
  settings:
    wrapcheck:
      extra-ignore-sigs: &wrap-ignore-sigs [errx.New(, gofi.Context).Next(]
      ignore-package-globs: &wrap-ignore-packages ["*/internal/http/httperr"]
    custom:
      wrapgaps:
        type: module
        description: Errors returned directly in a closure, or from a generic call, are wrapped (ERR-3)
        settings:
          extra-ignore-sigs: *wrap-ignore-sigs
          ignore-package-globs: *wrap-ignore-packages
```

## Build and test

```sh
go test ./...                 # analysistest cases in testdata/src
cd .. && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 custom # reads lint/.custom-gcl.yml; writes lint/bin/golangci-lint
```

The module path, `github.com/michaelolof/wrapgaps`, is a placeholder until the module is published. `.custom-gcl.yml` builds it from the local `path`, so the module path doesn't need to resolve.
