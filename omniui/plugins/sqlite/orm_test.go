package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

type testTask struct {
	ID    uint   `gorm:"primaryKey"`
	Title string `gorm:"uniqueIndex;not null"`
	Done  bool
}

func TestORMCRUDAndRawShareDatabase(t *testing.T) {
	db, err := Open(t.Context(), ":memory:", Options{Models: []any{&testTask{}}, MaxRows: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tasks := Table[testTask](db)
	first := testTask{Title: "first", Done: true}
	if err := tasks.Create(t.Context(), &first); err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 {
		t.Fatal("primary key missing")
	}
	if err := tasks.Update(t.Context(), first.ID, map[string]any{"done": false, "title": ""}); err != nil {
		t.Fatal(err)
	}
	got, err := tasks.Get(t.Context(), first.ID)
	if err != nil || got.Done || got.Title != "" {
		t.Fatal(got, err)
	}
	rows, err := db.Query(t.Context(), "SELECT count(*) FROM test_tasks")
	if err != nil || rows.Values[0][0] != int64(1) {
		t.Fatal(rows, err)
	}
	if _, err := tasks.Get(t.Context(), "1 OR 1=1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("ID interpreted as SQL", err)
	}
	if err := tasks.Delete(t.Context(), "1 OR 1=1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("delete interpreted ID as SQL", err)
	}
	for _, title := range []string{"two", "three"} {
		item := testTask{Title: title}
		if err := tasks.Create(t.Context(), &item); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tasks.List(t.Context()); !errors.Is(err, ErrRowLimit) {
		t.Fatal(err)
	}
	page, err := tasks.List(t.Context(), ListOptions{Limit: 1, Offset: 1, OrderBy: "id"})
	if err != nil || len(page) != 1 || page[0].Title != "two" {
		t.Fatal(page, err)
	}
	filtered, err := tasks.List(t.Context(), ListOptions{Where: "title = ?", Args: []any{"three"}})
	if err != nil || len(filtered) != 1 {
		t.Fatal(filtered, err)
	}
	if err := tasks.Delete(t.Context(), first.ID); err != nil {
		t.Fatal(err)
	}
	if err := tasks.Update(t.Context(), first.ID, map[string]any{"title": "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := tasks.Get(nil, first.ID); err == nil {
		t.Fatal("nil context accepted")
	}
}
func TestORMTransactionCancellationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tasks.db")
	db, err := Open(t.Context(), path, Options{Models: []any{&testTask{}}, CreateDirectories: true, WAL: true})
	if err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("rollback")
	if err := db.WithTransaction(t.Context(), func(tx *gorm.DB) error {
		if err := tx.Create(&testTask{Title: "discard"}).Error; err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	items, err := Table[testTask](db).List(t.Context())
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic not propagated")
			}
		}()
		_ = db.WithTransaction(t.Context(), func(tx *gorm.DB) error {
			if err := tx.Create(&testTask{Title: "panic"}).Error; err != nil {
				t.Fatal(err)
			}
			panic("rollback")
		})
	}()
	if err := db.WithTransaction(t.Context(), func(tx *gorm.DB) error { return tx.Create(&testTask{Title: "kept"}).Error }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Table[testTask](db).List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Table[testTask](db).List(t.Context()); err == nil {
		t.Fatal("closed pool used")
	}
	db, err = Open(t.Context(), path, Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	items, err = Table[testTask](db).List(t.Context())
	if err != nil || len(items) != 1 || items[0].Title != "kept" {
		t.Fatal(items, err)
	}
	if err := Table[testTask](db).Create(t.Context(), &testTask{Title: "forbidden"}); err == nil {
		t.Fatal("read-only wrote")
	}
}
