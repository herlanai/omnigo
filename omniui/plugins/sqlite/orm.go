package sqlite

import (
	"context"
	"errors"

	dialect "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrNotFound = gorm.ErrRecordNotFound
var ErrRowLimit = errors.New("sqlite: row limit exceeded")

func (d *DB) initializeORM(ctx context.Context, models []any) error {
	// Supply the already-open modernc connection. The dialect must never open its
	// default sqlite3 driver; production works with CGO_ENABLED=0.
	orm, err := gorm.Open(dialect.New(dialect.Config{DriverName: "sqlite", Conn: d.db}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true})
	if err != nil {
		return err
	}
	d.orm = orm
	if len(models) > 0 {
		return d.Migrate(ctx, models...)
	}
	return ctx.Err()
}

// ORM returns an independent GORM session bound to ctx. It uses the same pool as
// Execute/Query. Callers must not close its underlying pool; DB owns it.
// Do not use this root session inside WithTransaction; use the supplied tx.
func (d *DB) ORM(ctx context.Context) *gorm.DB {
	session := d.orm.Session(&gorm.Session{NewDB: true, Context: ctx})
	if ctx == nil {
		session.AddError(errors.New("sqlite: nil context"))
	}
	return session
}
func (d *DB) Migrate(ctx context.Context, models ...any) error {
	if ctx == nil {
		return errors.New("sqlite: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.ORM(ctx).AutoMigrate(models...)
}

// WithTransaction commits only on a nil callback result; errors/panics roll back.
// The callback must use tx, not d.ORM or the raw SQL methods (single-connection pool).
func (d *DB) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if ctx == nil {
		return errors.New("sqlite: nil context")
	}
	if fn == nil {
		return errors.New("sqlite: nil transaction callback")
	}
	return d.ORM(ctx).Transaction(fn)
}
