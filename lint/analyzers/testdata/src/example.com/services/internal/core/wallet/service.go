package wallet

type Service struct{}

func New(store any, opts ...string) *Service { return &Service{} } // want `a service's constructor is func New\(d Deps\) \(\*Service, error\), and it checks the required dependencies \(ABS-5\)`
