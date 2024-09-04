package pg

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

const StoreMigrationRelativePath = "internal/store/pg/migrations"

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
		log.Fatalf("cannot create postgres driver: %v", err)
	}

	// get current dir and try to construct the abs path to migrations directory
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot get current dir")
	}
	migrationsDir := removeOverlap(curDir, StoreMigrationRelativePath)

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir, "postgres", driver)
	if err != nil {
		log.Fatalf("cannot create migrate instance (with migrations path: %s): %v", migrationsDir, err)
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

// splitPath splits a path into its components, handling the leading separator
func splitPath(p string) []string {
	// check if the path is absolute and needs special handling
	isAbsolute := strings.HasPrefix(p, string(filepath.Separator))
	if isAbsolute {
		p = p[1:]
	}

	// split the path into components and filter out empty parts
	parts := strings.Split(filepath.Clean(p), string(filepath.Separator))
	if isAbsolute {
		parts = append([]string{""}, parts...)
	}
	return parts
}

// joinPath joins path components into a single path, handling the leading separator
func joinPath(parts []string) string {
	if len(parts) > 0 && parts[0] == "" {
		return string(filepath.Separator) + filepath.Join(parts[1:]...)
	}
	return filepath.Join(parts...)
}

// findLongestOverlap finds the longest overlapping sequence of parts between curDirParts and relDirParts
func findLongestOverlap(curDirParts, relDirParts []string) (longestStart, longestEnd int) {
	longestStart = -1
	longestEnd = -1
	maxLen := 0

	for i := 0; i < len(curDirParts); i++ {
		for j := 0; j < len(relDirParts); j++ {
			k := 0
			// check for overlap
			for i+k < len(curDirParts) && j+k < len(relDirParts) && curDirParts[i+k] == relDirParts[j+k] {
				k++
			}
			// update if this is the longest overlap found
			if k > maxLen {
				maxLen = k
				longestStart = i
				longestEnd = i + k
			}
		}
	}
	return longestStart, longestEnd
}

// removeOverlap removes the longest overlapping sequence between curDir and relDir
func removeOverlap(curDir, relDir string) string {
	// split the directories into components
	curDirParts := splitPath(curDir)
	relDirParts := splitPath(relDir)

	// find the longest overlapping sequence of parts
	start, end := findLongestOverlap(curDirParts, relDirParts)
	if start != -1 && end != -1 {
		// remove the overlapping sequence from curDir and appends the relDir to build abs path
		remainingParts := curDirParts[:start]
		remainingParts = append(remainingParts, curDirParts[end:]...)
		remainingParts = append(remainingParts, relDir)
		return joinPath(remainingParts)
	}

	// if no overlap is found, return the original curDir plus the relDir
	return filepath.Join([]string{curDir, relDir}...)
}
