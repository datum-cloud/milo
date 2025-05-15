package role

import (
	"context"
	"database/sql"
	"fmt"
)

type Kind string

const (
	InheritedRoleKind Kind = "InheritedRole"
)

type DatabaseResolver func(ctx context.Context, kind Kind, roleName string) ([]string, error)

func DatabaseRoleResolver(db *sql.DB) (DatabaseResolver, error) {
	// Prepares a statement to find all roles that inherit from a given role name.
	inheritedRoleStmt, err := db.Prepare(`SELECT name FROM iam_datumapis_com_Role_resource WHERE data->'spec'->'inheritedRoles' @> $1::jsonb`)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, kind Kind, roleName string) ([]string, error) {
		var rows *sql.Rows
		var err error

		switch kind {
		case InheritedRoleKind:
			jsonArray := fmt.Sprintf(`["%s"]`, roleName)
			rows, err = inheritedRoleStmt.QueryContext(ctx, jsonArray)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
		default:
			return nil, fmt.Errorf("unsupported subject type provided: %s", string(kind))
		}

		var roles []string
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				return nil, err
			}
			roles = append(roles, role)
		}

		if err := rows.Err(); err != nil {
			return nil, err
		}

		return roles, nil
	}, nil
}
