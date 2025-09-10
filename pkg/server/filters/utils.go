package filters

import (
	"maps"

	"k8s.io/apiserver/pkg/authentication/user"
)

// Returns a copy of the given user.DefaultInfo with Extra data safely added
//
// User structs are cached in the auth layer and can be accessed concurrently
// across requests, so we need to perform a shallow copy and add extra info
// into the new struct.
func userWithExtra(u *user.DefaultInfo, extra map[string][]string) *user.DefaultInfo {
	uCopy := *u
	uCopy.Extra = make(map[string][]string, len(u.Extra)+len(extra))
	maps.Copy(uCopy.Extra, u.Extra)
	maps.Copy(uCopy.Extra, extra)

	return &uCopy
}
