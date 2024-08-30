package pg

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ethereum-fetcher/cmd"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/v4/boil"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // fixes: unknown driver 'file'
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const StoreMigrationRelativePath = "file://./internal/store/pg/migrations"

// NewStore provides a store implementation through *pg.Store type
func NewStore(ctx context.Context, v *viper.Viper) (*Store, error) {
	var db *sql.DB
	var err error

	// as of Go 1.23, the garbage collector can recover unreferenced
	// tickers, so no need to defer retryTimer.Stop()
	retryTimer := time.NewTicker(10 * time.Second)

	// make sure the database container is up and running (accepts connections)
	for {
		db, err = sql.Open("pgx", v.GetString(cmd.DBConnectionURL))
		if err == nil {
			// Query the database to early catch connectivity issues
			if err = db.Ping(); err == nil {
				// we have a valid connection
				break
			}
		}

		select {
		case <-retryTimer.C:
			return nil, err
		case <-ctx.Done():
			// return nil error: don't need to panic the caller since we have a shutdown request
			return nil, nil
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// max 25 idle connections
	db.SetMaxIdleConns(25)
	// max 25 open connections
	db.SetMaxOpenConns(25)
	// max lifetime of connection
	db.SetConnMaxLifetime(10 * time.Minute)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("could not create postgres driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(StoreMigrationRelativePath, "postgres", driver)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}

	// Run the migration
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migration failed: %v", err)
	} else if errors.Is(err, migrate.ErrNoChange) {
		log.Info("no migration needed, database is up to date")
	} else {
		log.Info("migration completed successfully")
	}

	// set the database connection for SQLBoiler
	boil.SetDB(db)

	s := &Store{
		ctx: ctx,
		db:  db,
	}

	return s, nil
}
