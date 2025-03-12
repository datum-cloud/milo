package postgres

import (
	"database/sql"
	"fmt"

	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/storage/otelstorage"
)

type Service struct {
	Resources map[string]storage.Resource
}

// ResourceServer provides a implementation of the `storage.ResourceServer` that
// uses a Postgres compatible SQL server. The Postgres database will have tables
// automatically configured for the resource provided.
func ResourceServer[R storage.Resource](db *sql.DB, resource R) (storage.ResourceServer[R], error) {
	resourceStorage := &databaseStorage[R]{
		database: db,
		zero:     resource,
	}

	// Confirm that the table exists in the database
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			uid                  UUID NOT NULL PRIMARY KEY,
			name                 TEXT NOT NULL,
			parent               TEXT NOT NULL,
			data                 JSONB NOT NULL,
			CONSTRAINT %s_resource_name_unique UNIQUE (name)
		)`,
		resourceTableName(resource),
		resourceTableName(resource),
	))
	if err != nil {
		return nil, err
	}

	return otelstorage.WithTracing(resourceStorage), nil
}
