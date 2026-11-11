package deliveries

type Repo struct{}

func New() *Repo { return &Repo{} }

func (r *Repo) ClaimDelivery() error { return nil } // want `repeats the package's name, "delivery"`

func (r *Repo) MarkSent() error { return nil }
