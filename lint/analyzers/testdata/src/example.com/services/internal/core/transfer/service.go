package transfer

type Store interface{ Get() error }

type Deps struct{ Store Store }

type Service struct {
	store   Store
	Metrics any // want `Service has no exported fields: pass Metrics through Deps \(ABS-5\)`
}

func New(d Deps) (*Service, error) { return &Service{store: d.Store}, nil }

func NewWithDefaults() *Service { return &Service{} } // want `a service is built only by New\(Deps\): remove NewWithDefaults \(ABS-5\)`

func (s *Service) WithStore(st Store) *Service { s.store = st; return s } // want `a service is configured only through New\(Deps\): remove WithStore \(ABS-5\)`

func (s *Service) Within() bool { return s.store != nil }
