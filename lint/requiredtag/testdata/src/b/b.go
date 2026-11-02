package b

type Remote struct {
	ID     string `lint:"required"`
	secret string `lint:"required"`
}

func New(id string) Remote {
	return Remote{ID: id} // want `Remote is missing required field secret`
}
