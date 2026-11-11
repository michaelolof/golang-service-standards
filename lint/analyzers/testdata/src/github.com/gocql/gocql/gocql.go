package gocql

type UUID [16]byte

type Query struct{}

type Session struct{}

func (s *Session) Query(stmt string, values ...any) *Query { return nil }
