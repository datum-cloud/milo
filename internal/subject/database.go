package subject

import (
	"context"
	"database/sql"
	"fmt"
)

func DatabaseResolver(db *sql.DB) (Resolver, error) {
	userStmt, err := db.Prepare("SELECT name FROM iam_datumapis_com_User_resource WHERE data->'spec'->>'email' = $1")
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, subjectIdentifier string) (*SubjectReference, error) {
		var row *sql.Row

		subjectName, subjectKind, err := Parse(subjectIdentifier)
		if err != nil {
			return nil, err
		}

		switch subjectKind {
		case UserKind:
			row = userStmt.QueryRowContext(ctx, subjectName)
		default:
			return nil, fmt.Errorf("unsupported subject type provided: %s", string(subjectKind))
		}

		if row.Err() == sql.ErrNoRows {
			return nil, ErrSubjectNotFound
		} else if row.Err() != nil {
			return nil, row.Err()
		}

		var subject string
		if err := row.Scan(&subject); err == sql.ErrNoRows {
			return nil, ErrSubjectNotFound
		} else if err != nil {
			return nil, err
		}

		return &SubjectReference{
			ResourceName:      subject,
			Kind:              subjectKind,
			SubjectIdentifier: subjectIdentifier,
		}, nil
	}, nil
}
