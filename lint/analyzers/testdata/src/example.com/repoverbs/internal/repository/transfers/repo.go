package transfers

import "context"

type Repo struct{}

func New() *Repo { return &Repo{} }

func (r *Repo) Get(ctx context.Context) error             { return nil }
func (r *Repo) CreateOrGet(ctx context.Context) error     { return nil }
func (r *Repo) CancelInitiated(ctx context.Context) error { return nil }
func (r *Repo) Insert(ctx context.Context) error          { return nil } // want `Insert isn't a repository verb: use Create \(DATA-3\)`
func (r *Repo) FetchByKey(ctx context.Context) error      { return nil } // want `Fetch isn't a repository verb: use Get or Find \(DATA-3\)`
func (r *Repo) GetTransfer(ctx context.Context) error     { return nil } // want `Repo.GetTransfer repeats the package's name, "transfer": transfers.Repo already says it \(DATA-3\)`
func (r *Repo) ListTransfers(ctx context.Context) error   { return nil } // want `repeats the package's name, "transfers"`
func (r *Repo) load(ctx context.Context) error            { return nil }
