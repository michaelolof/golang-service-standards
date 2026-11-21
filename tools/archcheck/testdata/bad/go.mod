module example.com/bad

go 1.26

require (
	github.com/jackc/pgx/v5 v5.0.0
	github.com/michaelolof/errx v0.0.0
	github.com/michaelolof/gofi v0.0.0
)

replace github.com/michaelolof/errx => ./third_party/errx

replace github.com/michaelolof/gofi => ./third_party/gofi

replace github.com/jackc/pgx/v5 => ./third_party/pgx
