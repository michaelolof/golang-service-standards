# requiredtag

A golangci-lint module plugin that reports struct literals leaving out a field marked required by a struct tag (STY-3).

Fields are optional, as in plain Go. A field becomes required only when one of the configured tags marks it:

```go
type CancelArgs struct {
	TenantID   domain.TenantID   `lint:"required"`
	TransferID domain.TransferID `lint:"required"`
	Reason     string
}

svc.Cancel(ctx, CancelArgs{TenantID: t})
// CancelArgs is missing required field TransferID (requiredtag)
```

## Configuration

```yaml
linters:
  enable: [requiredtag]
  settings:
    custom:
      requiredtag:
        type: module
        description: Struct literals set every field a struct tag marks required (STY-3)
        settings:
          tags: [validate:required, lint:required] # key:option
          allow-empty-returns: true                # return T{}, err and return T{}, false
```

A marker `key:option` matches when the value of tag `key`, split on commas, contains `option`:

| Tag | `validate:required` matches? |
|---|---|
| `validate:"required"` | yes |
| `validate:"required,email"` | yes |
| `validate:"omitempty"` | no |
| `validate:"required_if=Kind card"` | no |

## What it checks and what it doesn't

Checked:
- every keyed literal, `T{…}` and `&T{…}`, including elided ones in slices and maps;
- generic types, aliases, and types from other packages. Tags are read from type information, so the declaring package doesn't matter.

Skipped:
- positional literals, because the compiler already requires every field in them;
- unexported fields of another package's struct, which only that package can set;
- `return T{}, err` and `return T{}, false`, when `allow-empty-returns` is on.

Not seen, because no literal is involved: `var t T`, `new(T)`, and fields assigned after the literal. Build the value in one literal instead (STY-9).

## Build and test

```sh
go test ./...                 # analysistest cases in testdata/src
cd .. && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 custom # reads lint/.custom-gcl.yml; writes lint/bin/golangci-lint
```

The module path, `github.com/michaelolof/requiredtag`, is a placeholder until the module is published. `.custom-gcl.yml` builds it from the local `path`, so the module path doesn't need to resolve.
