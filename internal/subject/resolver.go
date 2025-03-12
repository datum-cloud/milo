package subject

import (
	"context"
	"errors"
	"strings"
)

// Resolver can resolve a subject reference to the ID of the subject that can be
// used when referencing the subject.
type Resolver func(ctx context.Context, subjectType Kind, subject string) (string, error)

type Kind string

const (
	UserKind           Kind = "iam.datumapis.com/User"
	ServiceAccountKind Kind = "iam.datumapis.com/ServiceAccount"
)

var ErrInvalidSubject = errors.New("invalid subject name format")
var ErrSubjectNotFound = errors.New("subject not found")

func Parse(subject string) (string, Kind, error) {
	parts := strings.Split(subject, ":")
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
