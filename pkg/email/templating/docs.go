package templating

import (
	"regexp"
)

var (
	// matches Go template variable references like {{ .UserName }}
	// Captures the full variable path after the leading dot; the first segment before a dot
	// is treated as the variable name that must be declared.
	templateVarRegexp = regexp.MustCompile(`\{\{\s*\.([A-Za-z][A-Za-z0-9_]*)`)
)
