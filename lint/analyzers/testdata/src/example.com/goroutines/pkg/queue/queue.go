// A pkg/ library owns its own goroutines.
package queue

func Start(work func()) { go work() }
