module example.com/good

go 1.26

ignore ./tools/vendored

require (
	github.com/jackc/pgx/v5 v5.0.0
	github.com/michaelolof/errx v0.0.0
	github.com/michaelolof/gofi v0.0.0
)

replace github.com/michaelolof/errx => ../bad/third_party/errx

replace github.com/michaelolof/gofi => ../bad/third_party/gofi

replace github.com/jackc/pgx/v5 => ../bad/third_party/pgx
