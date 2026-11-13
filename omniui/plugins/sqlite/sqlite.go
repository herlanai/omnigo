// Package sqlite is an optional pure-Go database service. Importing baseui does
// not register or link this driver. Callers own each DB and must close it.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type Options struct {
	ReadOnly    bool
	BusyTimeout time.Duration
	MaxRows     int
	// Models are migrated once during Open. Read-only connections cannot migrate.
	Models            []any
	WAL               bool
	CreateDirectories bool
}
type DB struct {
	db      *sql.DB
	orm     *gorm.DB
	maxRows int
}
type Result struct{ LastInsertID, RowsAffected int64 }
type Rows struct {
	Columns []string
	Values  [][]any
}
type Statement struct {
	SQL    string
	Params []any
}

func Open(ctx context.Context, path string, o Options) (*DB, error) {
	if ctx == nil {
		return nil, errors.New("sqlite: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.ReadOnly && (len(o.Models) > 0 || o.WAL || o.CreateDirectories) {
		return nil, errors.New("sqlite: read-only options cannot migrate, enable WAL or create directories")
	}
	if path == "" {
		return nil, errors.New("sqlite: empty database path")
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	if o.WAL && path != ":memory:" {
		q.Add("_pragma", "journal_mode(WAL)")
	}
	if o.BusyTimeout < 0 {
		return nil, errors.New("sqlite: negative busy timeout")
	}
	if o.BusyTimeout == 0 {
		o.BusyTimeout = 5 * time.Second
	}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", o.BusyTimeout.Milliseconds()))
	dsn := ""
	if path == ":memory:" {
		if o.ReadOnly {
			return nil, errors.New("sqlite: read-only in-memory database")
		}
		dsn = ":memory:?" + q.Encode()
	} else {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if o.CreateDirectories {
			if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
				return nil, err
			}
		}
		if o.ReadOnly {
			q.Set("mode", "ro")
		}
		u := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute), RawQuery: q.Encode()}
		dsn = u.String()
	}
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)
	if err = raw.PingContext(ctx); err != nil {
		raw.Close()
		return nil, err
	}
	if o.MaxRows <= 0 {
		o.MaxRows = 10000
	}
	db := &DB{db: raw, maxRows: o.MaxRows}
	if err := db.initializeORM(ctx, o.Models); err != nil {
		raw.Close()
		return nil, err
	}
	return db, nil
}
func (d *DB) Close() error { return d.db.Close() }
func result(r sql.Result, e error) (Result, error) {
	if e != nil {
		return Result{}, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return Result{}, e
	}
	n, e := r.RowsAffected()
	return Result{id, n}, e
}
func (d *DB) Execute(ctx context.Context, query string, args ...any) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("sqlite: nil context")
	}
	r, e := d.db.ExecContext(ctx, query, args...)
	return result(r, e)
}
func (d *DB) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	if ctx == nil {
		return Rows{}, errors.New("sqlite: nil context")
	}
	rows, e := d.db.QueryContext(ctx, query, args...)
	if e != nil {
		return Rows{}, e
	}
	defer rows.Close()
	columns, e := rows.Columns()
	if e != nil {
		return Rows{}, e
	}
	out := Rows{Columns: columns, Values: make([][]any, 0)}
	for rows.Next() {
		if len(out.Values) >= d.maxRows {
			return Rows{}, errors.New("sqlite: row limit exceeded")
		}
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if e = rows.Scan(pointers...); e != nil {
			return Rows{}, e
		}
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = append([]byte(nil), b...)
			}
		}
		out.Values = append(out.Values, values)
	}
	return out, rows.Err()
}
func (d *DB) Transaction(ctx context.Context, statements []Statement) ([]Result, error) {
	if ctx == nil {
		return nil, errors.New("sqlite: nil context")
	}
	tx, e := d.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	out := make([]Result, 0, len(statements))
	for _, s := range statements {
		r, e := tx.ExecContext(ctx, s.SQL, s.Params...)
		v, e := result(r, e)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}
