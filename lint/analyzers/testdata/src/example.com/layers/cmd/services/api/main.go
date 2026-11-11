package main

import (
	"example.com/layers/internal/adapters/kafka"
	"example.com/layers/internal/config"
	"example.com/layers/internal/testutils/harness" // want `internal/testutils/harness is test code`
)

var (
	_ = kafka.Name
	_ config.Mongo
	_ = harness.Name
)

func main() {}
