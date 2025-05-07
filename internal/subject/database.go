package subject

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func DatabaseResolver(db *sql.DB) (Resolver, error) {
	userStmt, err := db.Prepare("SELECT name FROM iam_datumapis_com_User_resource WHERE data->'spec'->>'email' = $1")
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, kind Kind, subjectName string) (string, error) {
		var row *sql.Row

		switch kind {
		case UserKind:
			row = userStmt.QueryRowContext(ctx, subjectName)
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

		switch kind {
		case UserKind:
			return strings.TrimPrefix(subject, "users/"), nil
		default:
			return "", fmt.Errorf("unsupported subject type provided: %s", string(kind))
		}

	}, nil
}
