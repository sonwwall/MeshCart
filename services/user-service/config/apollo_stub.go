package config

import "errors"

// StubApolloLoader is a placeholder for future Apollo integration.
// Replace this with a real Apollo client implementation later.
type StubApolloLoader struct{}

func (l *StubApolloLoader) Load() (Config, error) {
	return Config{}, errors.New("apollo loader is not implemented")
}
