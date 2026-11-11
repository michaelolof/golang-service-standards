// Package config reads the environment.
package config

// Mongo is the database section.
type Mongo struct {
	URI      string `env:"MONGO_URI,required"`
	Database string `env:"MONGO_DATABASE"`
}

// Log is the logging section.
type Log struct {
	Level string `env:"LOG_LEVEL" envDefault:"info"`
}
