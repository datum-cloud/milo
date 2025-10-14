package version

import (
	"fmt"
	"strconv"
	"strings"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

// IsVersionHigher returns true if newVersion is strictly greater than prevVersion using semantic versioning rules.
// Both versions must follow the `vMAJOR.MINOR.PATCH` format (validated by kubebuilder tag on DocumentVersion).
// If either version is invalid the function returns an error.
func IsVersionHigher(newVersion, prevVersion documentationv1alpha1.DocumentVersion) (bool, error) {
	newParts, err := parseSemver(string(newVersion))
	if err != nil {
		return false, fmt.Errorf("invalid new version: %w", err)
	}
	prevParts, err := parseSemver(string(prevVersion))
	if err != nil {
		return false, fmt.Errorf("invalid previous version: %w", err)
	}

	for i := 0; i < 3; i++ {
		if newParts[i] > prevParts[i] {
			return true, nil
		}
		if newParts[i] < prevParts[i] {
			return false, nil
		}
	}
	// versions are equal
	return false, nil
}

// parseSemver converts a string in form vMAJOR.MINOR.PATCH into a slice of three integers.
func parseSemver(v string) ([3]int, error) {
	if !strings.HasPrefix(v, "v") {
		return [3]int{}, fmt.Errorf("version must start with 'v'")
	}
	v = strings.TrimPrefix(v, "v")
	segments := strings.Split(v, ".")
	if len(segments) != 3 {
		return [3]int{}, fmt.Errorf("version must have three segments")
	}
	var parts [3]int
	for i, s := range segments {
		n, err := strconv.Atoi(s)
		if err != nil {
			return [3]int{}, fmt.Errorf("invalid segment %q: %w", s, err)
		}
		parts[i] = n
	}
	return parts, nil
}
