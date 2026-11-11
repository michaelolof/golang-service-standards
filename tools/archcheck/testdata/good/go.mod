module example.com/good

go 1.26

require github.com/michaelolof/errx v0.0.0

replace github.com/michaelolof/errx => ../bad/third_party/errx
