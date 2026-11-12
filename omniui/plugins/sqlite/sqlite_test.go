package sqlite

import (
	"context"
	"testing"
)

func TestRealDatabaseRollbackTypesAndClose(t *testing.T) {
	ctx := context.Background()
	db, e := Open(ctx, ":memory:", Options{MaxRows: 2})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Execute(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY, value BLOB)"); e != nil {
		t.Fatal(e)
	}
	_, e = db.Transaction(ctx, []Statement{{"INSERT INTO items VALUES(?,?)", []any{1, []byte{0, 1}}}, {"INSERT INTO items VALUES(?,?)", []any{1, "duplicate"}}})
	if e == nil {
		t.Fatal("duplicate committed")
	}
	rows, e := db.Query(ctx, "SELECT * FROM items")
	if e != nil || len(rows.Values) != 0 {
		t.Fatal(rows, e)
	}
	_, e = db.Execute(ctx, "INSERT INTO items VALUES(?,?)", 1, []byte{0, 1})
	if e != nil {
		t.Fatal(e)
	}
	rows, e = db.Query(ctx, "SELECT id,value,NULL FROM items")
	if e != nil || rows.Values[0][0] != int64(1) || rows.Values[0][2] != nil {
		t.Fatal(rows, e)
	}
	fk, e := db.Query(ctx, "PRAGMA foreign_keys")
	if e != nil || fk.Values[0][0] != int64(1) {
		t.Fatal("foreign keys disabled", e)
	}
	if _, e = db.Query(ctx, "SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3"); e == nil {
		t.Fatal("unbounded query")
	}
	db.Close()
	if _, e = db.Execute(ctx, "SELECT 1"); e == nil {
		t.Fatal("closed database used")
	}
}
