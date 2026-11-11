// Package config reads the environment.
package config

// Mongo is the database section.
type Mongo struct {
	URI   string `env:"MONGO_URI,required"`
	Level string `env:"LOG_LEVEL" envDefault:"info"`
}
