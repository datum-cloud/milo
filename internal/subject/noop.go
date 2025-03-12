package subject

import "context"

func NoopResolver() Resolver {
	return func(_ context.Context, _ Kind, name string) (string, error) {
		return name, nil
	}
}
