package export

func dispatch(send func()) {
	go send() // want `a goroutine needs an owner that waits for it: use an errgroup with SetLimit, a component under supervise.Run, or a durable job \(CON-1, CON-2\)`

	go func() { // want `a goroutine needs an owner`
		send()
	}()
}

var _ = dispatch
