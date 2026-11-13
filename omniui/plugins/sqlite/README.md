# SQLite + GORM

An optional database plugin for OmniGo applications. Open a database, declare
models, and use typed CRUD operations. GORM and raw SQL share one owned connection
pool; importing `baseui` alone does not include this plugin.

## Start with a model

```go
package main

import (
    "context"
    "log"

    "github.com/herlanai/omnigo/omniui/plugins/sqlite"
)

type Task struct {
    ID    uint `gorm:"primaryKey"`
    Title string
    Done  bool
}

func main() {
    ctx := context.Background()
    db, err := sqlite.Open(ctx, "data/tasks.db", sqlite.Options{
        Models: []any{&Task{}}, CreateDirectories: true, WAL: true,
    })
    if err != nil { log.Fatal(err) }
    defer db.Close()

    tasks := sqlite.Table[Task](db)
    task := Task{Title: "Build with OmniGo"}
    if err := tasks.Create(ctx, &task); err != nil { log.Fatal(err) }
    if err := tasks.Update(ctx, task.ID, map[string]any{"done": true}); err != nil { log.Fatal(err) }
    rows, err := tasks.List(ctx, sqlite.ListOptions{OrderBy: "id", Limit: 20})
    if err != nil { log.Fatal(err) }
    log.Printf("tasks: %+v", rows)
}
```

Use an application-owned writable data directory in production, rather than a
path relative to the executable. `:memory:` creates an isolated temporary database.
Opening a database does not register an OmniWeb endpoint or launch a goroutine.

## API

| Operation | API |
| --- | --- |
| Open and migrate | `Open(ctx, path, Options{Models: []any{&Task{}}})` |
| Migrate later | `db.Migrate(ctx, &Task{})` |
| Insert and populate generated ID | `sqlite.Table[Task](db).Create(ctx, &task)` |
| Find by primary key | `tasks.Get(ctx, id)` |
| Update including false, zero and empty strings | `tasks.Update(ctx, id, map[string]any{"done": false})` |
| Delete by primary key | `tasks.Delete(ctx, id)` |
| Filter and paginate | `tasks.List(ctx, sqlite.ListOptions{Where: "done = ?", Args: []any{false}, OrderBy: "id", Limit: 20, Offset: 20})` |
| Full GORM API | `db.ORM(ctx).Preload(...).Find(...)` |
| GORM transaction | `db.WithTransaction(ctx, func(tx *gorm.DB) error { ... })` |
| Existing SQL API | `db.Execute`, `db.Query`, `db.Transaction` |

`Table[T]` expects a struct model and a live DB returned by `Open`. CRUD key helpers
support a single primary key; use GORM conditions for composite keys. Missing keys
return `sqlite.ErrNotFound`; an empty list is successful. GORM conventions, hooks,
associations and soft deletion apply. Pass trusted model field names in update
maps. SQL fragments in `Where` must be application-owned; supply user values via
`Args` or map conditions. `OrderBy` is quoted as a column name, not SQL.

`List` defaults to `MaxRows` (10,000) and returns `ErrRowLimit` if more rows exist.
An explicit positive `Limit` requests a page and must not exceed `MaxRows`.
`Offset` must be nonnegative. Use a stable ordering for pagination. The limit also
applies to raw `Query`, but **does not constrain direct GORM queries**.

## Transactions and lifetime

```go
err := db.WithTransaction(ctx, func(tx *gorm.DB) error {
    if err := tx.Create(&Task{Title: "First"}).Error; err != nil {
        return err
    }
    return tx.Create(&Task{Title: "Second"}).Error
})
```

Returning an error rolls back; panics roll back and propagate. Inside the callback,
use only `tx`, including for reads: the pool has one connection to keep in-memory
state consistent and serialize SQLite writes. Calling `db.ORM`, a repository, or
raw methods from inside that transaction would wait for the occupied connection.

Every operation accepts a non-nil context. `ORM(ctx)` returns a fresh session;
query conditions do not leak between calls. GORM logging is silent by default;
advanced callers can configure a session logger. The application owns `DB.Close`;
do not close the underlying GORM pool separately.

Run database work off the UI goroutine. In a native content window, use
`WindowEnvironment.RunTask` and update control state only in its returned UI
callback. Cancel pending work when the window/application closes and wait for it
before closing the database. See [Studio's demo](../../../examples/studio/internal/demos/sqlite.go).

## Connection options

- Foreign keys are enabled on every connection; busy timeout defaults to 5 seconds.
- `CreateDirectories` creates missing parent directories with mode `0700` where
  supported; it does not change existing permissions or encrypt the database.
- `WAL` opts file databases into write-ahead logging; memory databases ignore it.
  SQLite can create adjacent `-wal` and `-shm` files. This does not add parallel
  writes or establish a measured performance gain.
- `ReadOnly` opens an existing file without write access. It cannot be combined
  with `Models`, `WAL`, or `CreateDirectories`. Run migrations before opening it
  read-only. Automatic migration is convenient for simple schemas, not a versioned
  production data migration system.

The implementation supplies the existing **modernc.org/sqlite** connection to
GORM's official SQLite dialect. Actual SQL works with `CGO_ENABLED=0`. The official
dialect also imports `mattn/go-sqlite3` transitively; its CGO-disabled stub is not
used as this plugin's driver. Dependency licenses are in [../licenses](../licenses).

## Studio demo and validation

Open **Plugins → SQLite** in Studio, then **Open demo database**. Add tasks,
toggle completion, filter titles, delete rows, and test a rolled-back insert.
Every demo window owns its own in-memory database, discarded on close.

```sh
CGO_ENABLED=0 go test ./omniui/plugins/sqlite
CGO_ENABLED=0 go test ./examples/studio/internal/demos -run TestSQLite
```

The demo uses real SQL and GORM, not mock rows. It does not expose a database to
Web clients; an OmniWeb bridge and equivalent performance benchmarks remain outside
this implementation's scope.
