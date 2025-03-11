package subject

import (
	"context"
	"database/sql"
	"fmt"
)

func DatabaseResolver(db *sql.DB) (Resolver, error) {
	userStmt, err := db.Prepare("SELECT id FROM users WHERE email = $1")
	if err != nil {
		return nil, err
	}

	serviceAccountStmt, err := db.Prepare("SELECT uid FROM iam_datumapis_com_serviceaccount_resource WHERE data->>'serviceAccountId' = $1")
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, kind Kind, subjectName string) (string, error) {
		var row *sql.Row

		switch kind {
		case UserKind:
			row = userStmt.QueryRowContext(ctx, subjectName)
		case ServiceAccountKind:
			row = serviceAccountStmt.QueryRowContext(ctx, subjectName)
		default:
			return "", fmt.Errorf("unsupported subject type provided: %s", string(kind))
		}

		if row.Err() == sql.ErrNoRows {
			return "", ErrSubjectNotFound
		} else if row.Err() != nil {
			return "", row.Err()
		}

		var subject string
		if err := row.Scan(&subject); err == sql.ErrNoRows {
			return "", ErrSubjectNotFound
		} else if err != nil {
			return "", err
		}

		return subject, nil
	}, nil
}
