package sqlite

import (
	"context"
	"errors"
	"fmt"
	"math"

	"gorm.io/gorm/clause"
)

// Repository wraps common model operations. Advanced joins/preloads/scopes stay
// available through DB.ORM rather than a second query language.
type Repository[T any] struct{ db *DB }

func Table[T any](db *DB) *Repository[T] { return &Repository[T]{db: db} }

type ListOptions struct {
	// Where accepts GORM map/struct conditions or a trusted SQL fragment with Args.
	// Never put user input into SQL fragments or expressions; pass it through Args.
	Where         any
	Args          []any
	OrderBy       string // Quoted as a column name, never treated as SQL.
	Descending    bool
	Limit, Offset int
}

func (r *Repository[T]) Create(ctx context.Context, value *T) error {
	if value == nil {
		return errors.New("sqlite: nil model")
	}
	return r.db.ORM(ctx).Create(value).Error
}
func (r *Repository[T]) Get(ctx context.Context, id any) (T, error) {
	var value T
	err := r.db.ORM(ctx).Where(clause.Eq{Column: clause.Column{Name: clause.PrimaryKey}, Value: id}).First(&value).Error
	return value, err
}

// Update uses a map so false, zero and empty strings are persisted. It never
// turns a missing key into an insert (unlike GORM Save).
func (r *Repository[T]) Update(ctx context.Context, id any, changes map[string]any) error {
	if len(changes) == 0 {
		return errors.New("sqlite: empty update")
	}
	result := r.db.ORM(ctx).Model(new(T)).Where(clause.Eq{Column: clause.Column{Name: clause.PrimaryKey}, Value: id}).Updates(changes)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
func (r *Repository[T]) Delete(ctx context.Context, id any) error {
	result := r.db.ORM(ctx).Where(clause.Eq{Column: clause.Column{Name: clause.PrimaryKey}, Value: id}).Delete(new(T))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// List defaults to the connection's MaxRows and errors if more records exist.
// An explicit positive Limit requests a page; Offset is nonnegative. OrderBy
// should name a stable column (usually the primary key) for predictable paging.
func (r *Repository[T]) List(ctx context.Context, options ...ListOptions) ([]T, error) {
	if len(options) > 1 {
		return nil, errors.New("sqlite: at most one list option")
	}
	var o ListOptions
	if len(options) == 1 {
		o = options[0]
	}
	if o.Limit < 0 || o.Offset < 0 || o.Limit > r.db.maxRows {
		return nil, fmt.Errorf("sqlite: invalid page bounds (maximum %d)", r.db.maxRows)
	}
	query := r.db.ORM(ctx)
	if o.Where != nil {
		query = query.Where(o.Where, o.Args...)
	}
	if o.OrderBy != "" {
		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: o.OrderBy}, Desc: o.Descending})
	}
	limit := o.Limit
	if limit == 0 {
		limit = r.db.maxRows
		if limit < math.MaxInt {
			limit++
		}
	}
	var values []T
	if err := query.Offset(o.Offset).Limit(limit).Find(&values).Error; err != nil {
		return nil, err
	}
	if len(values) > r.db.maxRows {
		return nil, ErrRowLimit
	}
	return values, nil
}
