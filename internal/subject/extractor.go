package subject

import "context"

// Extractor provides an interface for extracting an authenticated subject from
// a context.
type Extractor func(context.Context) (string, error)
