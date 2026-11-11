package supervise

func Run(fns ...func()) {
	for _, fn := range fns {
		go fn()
	}
}
