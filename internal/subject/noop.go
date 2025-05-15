package subject

import "context"

func NoopResolver() Resolver {
	return func(_ context.Context, subjectIdentifier string) (*SubjectReference, error) {
		subjectName, subjectKind, err := Parse(subjectIdentifier)
		if err != nil {
			return nil, err
		}

		return &SubjectReference{
			ResourceName:      subjectName,
			Kind:              subjectKind,
			SubjectIdentifier: subjectIdentifier,
		}, nil
	}
}
