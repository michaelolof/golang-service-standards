module example.com/good

go 1.26

require (
	github.com/michaelolof/errx v0.0.0
	github.com/michaelolof/gofi v0.0.0
)

replace github.com/michaelolof/errx => ../bad/third_party/errx

replace github.com/michaelolof/gofi => ../bad/third_party/gofi
