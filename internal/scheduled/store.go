package scheduled

import (
	"context"
	"fmt"

	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
)

func openMCPStore(ctx context.Context, sqlitePath string) (*sqlitestore.Store, error) {
	db, err := sqlitestore.OpenDB(ctx, sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := sqlitestore.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}

	return sqlitestore.New(db), nil
}
