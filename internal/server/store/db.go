package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // database/sql driver "sqlite"
)

// dialect selects SQL-flavor differences (placeholders, row locking).
type dialect int

const (
	dialectPostgres dialect = iota
	dialectSQLite
)

// Store wraps a database/sql handle. One implementation, driver chosen by DSN:
// pgx (Postgres, opt-in) or modernc sqlite (default). Query methods live in the
// per-domain files (users.go, vaults.go, ...).
type Store struct {
	db      *sql.DB
	dialect dialect
}

// New opens the database and pings it. driver is "pgx" or "sqlite".
func New(ctx context.Context, driver, dsn string) (*Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}
	d := dialectPostgres
	if driver == "sqlite" {
		d = dialectSQLite
		// SQLite tolerates one writer; serialize to avoid "database is locked".
		db.SetMaxOpenConns(1)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{db: db, dialect: d}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Ping checks liveness (health endpoint).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// rebind rewrites `?` placeholders to `$1..$N` for Postgres; passthrough for
// SQLite. Queries are authored with `?` and never contain a literal `?`.
func (s *Store) rebind(q string) string {
	if s.dialect != dialectPostgres {
		return q
	}
	var b strings.Builder
	b.Grow(len(q) + 8)
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

// forUpdate returns " FOR UPDATE" on Postgres, empty on SQLite (unsupported;
// MaxOpenConns(1) + optimistic version check cover correctness there).
func (s *Store) forUpdate() string {
	if s.dialect == dialectPostgres {
		return " FOR UPDATE"
	}
	return ""
}

// newID returns a random UUIDv4. Generated Go-side so ids are engine-agnostic
// (no gen_random_uuid / RETURNING dependency).
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
