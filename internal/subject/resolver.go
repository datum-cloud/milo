package subject

import (
	"context"
	"errors"
	"strings"
)

// Resolver can resolve a subject reference to the ID of the subject that can be
// used when referencing the subject.
type Resolver func(ctx context.Context, subjectIdentifier string) (*SubjectReference, error)

// SubjectReference is a reference to a subject that can be used to look up the
// subject in the resource storage system.
type SubjectReference struct {
	// The kind of the subject.
	Kind Kind

	// The resource name of the subject that can be used to look up the subject in
	// the resource storage system.
	ResourceName string

	// The identifier of the subject that can be used to reference the subject
	// across the system. This will be in the format
	// `<subjectType>:<subjectIdentifier>` (e.g. `user:john.doe@example.com`).
	SubjectIdentifier string
}

type Kind string

const (
	UserKind           Kind = "iam.datumapis.com/User"
	ServiceAccountKind Kind = "iam.datumapis.com/ServiceAccount"
)

var ErrInvalidSubject = errors.New("invalid subject name format")
var ErrSubjectNotFound = errors.New("subject not found")

func Parse(subjectIdentifier string) (string, Kind, error) {
	parts := strings.Split(subjectIdentifier, ":")
	var subjectType Kind

	switch parts[0] {
	case "user":
		subjectType = UserKind
	case "serviceAccount":
		subjectType = ServiceAccountKind
	case "allAuthenticatedUsers":
		return "*", UserKind, nil
	default:
		return "", "", ErrInvalidSubject
	}

	return parts[1], subjectType, nil
}
