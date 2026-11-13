package demos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/plugins/sqlite"
	"gorm.io/gorm"
)

// Business models and sample data stay in Studio; connection/ORM/CRUD plumbing
// belongs to the plugin. Each window owns an isolated, real SQLite database.
type sqliteTask struct {
	ID        uint   `gorm:"primaryKey"`
	Title     string `gorm:"uniqueIndex;not null"`
	Done      bool
	CreatedAt time.Time
}

func (sqliteTask) TableName() string { return "demo_tasks" }

type sqliteDemo struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	jobs                sync.WaitGroup
	closed, busy, ready bool // UI-owned
	done                chan struct{}
	title, filter       *ui.TextBuffer
	rows                []sqliteTask
	message             string
	db                  *sqlite.DB // Worker-owned; close only after jobs have returned.
}

func newSQLiteDemo() *sqliteDemo {
	ctx, cancel := context.WithCancel(context.Background())
	return &sqliteDemo{ctx: ctx, cancel: cancel, done: make(chan struct{}), title: ui.NewTextBuffer("Explore GORM with OmniGo"), filter: ui.NewTextBuffer(""), message: "Open an isolated database to begin."}
}
func (s *sqliteDemo) close() {
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
	go func() {
		s.jobs.Wait()
		if s.db != nil {
			_ = s.db.Close()
		}
		close(s.done)
	}()
}

func (d *Demo) sqliteAction(action func(context.Context, *sqlite.DB) (string, error)) {
	s := d.sqlite
	if s.busy || s.closed {
		return
	}
	s.busy = true
	filter := s.filter.String() // Snapshot UI buffers before leaving the UI goroutine.
	s.jobs.Add(1)
	work := func() func() {
		defer s.jobs.Done()
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		var err error
		message := "Loaded current rows"
		if s.db == nil {
			message = "Connected · models migrated · 3 sample rows"
			s.db, err = sqlite.Open(ctx, ":memory:", sqlite.Options{Models: []any{&sqliteTask{}}, MaxRows: 100})
			if err == nil {
				err = s.db.WithTransaction(ctx, func(tx *gorm.DB) error {
					return tx.Create(&[]sqliteTask{{Title: "Open SQLite without CGO", Done: true}, {Title: "Create models with GORM"}, {Title: "Try a transaction rollback"}}).Error
				})
			}
		}
		if err == nil && action != nil {
			message, err = action(ctx, s.db)
		}
		var rows []sqliteTask
		if err == nil {
			options := sqlite.ListOptions{Limit: 100, OrderBy: "id"}
			if filter != "" {
				options.Where = "title LIKE ?"
				options.Args = []any{"%" + filter + "%"}
			}
			rows, err = sqlite.Table[sqliteTask](s.db).List(ctx, options)
		}
		ready := s.db != nil
		return func() {
			if s.closed {
				return
			}
			s.busy = false
			s.ready = ready
			if err != nil {
				s.message = err.Error()
				return
			}
			s.rows = rows
			s.message = message
		}
	}
	if d.RunTask != nil {
		d.RunTask(work)
	} else {
		work()()
	} // Deterministic offline gallery/tests.
}
func (d *Demo) sqliteContent() *ui.Element {
	if d.sqlite == nil {
		d.sqlite = newSQLiteDemo()
	}
	s := d.sqlite
	rows := []*ui.Element{}
	for _, task := range s.rows {
		rows = append(rows, ui.Row(fmt.Sprintf("sqlite/row/%d", task.ID),
			ui.Checkbox(fmt.Sprintf("sqlite/done/%d", task.ID), task.Title, task.Done, func(done bool) {
				d.sqliteAction(func(ctx context.Context, db *sqlite.DB) (string, error) {
					return "Updated through Repository.Update", sqlite.Table[sqliteTask](db).Update(ctx, task.ID, map[string]any{"done": done})
				})
			}).Disabled(s.busy).Grow(1),
			ui.Text(fmt.Sprintf("sqlite/id/%d", task.ID), fmt.Sprintf("#%d", task.ID)),
			ui.Button(fmt.Sprintf("sqlite/delete/%d", task.ID), "Delete", func() {
				d.sqliteAction(func(ctx context.Context, db *sqlite.DB) (string, error) {
					return "Deleted through Repository.Delete", sqlite.Table[sqliteTask](db).Delete(ctx, task.ID)
				})
			}).Variant(ui.Outline).Disabled(s.busy),
		).Gap(12))
	}
	if len(rows) == 0 {
		rows = append(rows, ui.Text("sqlite/empty", "No rows to display."))
	}
	return ui.Column("sqlite/demo",
		ui.Text("sqlite/description", "GORM + SQLite · real database · pure-Go connection · isolated per window").Wrap(true),
		ui.Row("sqlite/connection", ui.Button("sqlite/open", "Open demo database", func() { d.sqliteAction(nil) }).Disabled(s.busy || s.ready), ui.Button("sqlite/refresh", "Refresh", func() { d.sqliteAction(nil) }).Variant(ui.Outline).Disabled(s.busy || !s.ready), ui.Badge("sqlite/state", map[bool]string{true: "Connected", false: "Not connected"}[s.ready])).Gap(12),
		ui.Row("sqlite/create", ui.Input("sqlite/title", "New task title", s.title).Grow(1).Disabled(s.busy), ui.Button("sqlite/add", "Add task", func() {
			title := strings.TrimSpace(s.title.String())
			if title == "" {
				s.message = "Enter a task title."
				return
			}
			d.sqliteAction(func(ctx context.Context, db *sqlite.DB) (string, error) {
				return "Created through Repository.Create", sqlite.Table[sqliteTask](db).Create(ctx, &sqliteTask{Title: title})
			})
		}).Disabled(s.busy || !s.ready)).Gap(12),
		ui.Row("sqlite/search", ui.Input("sqlite/filter", "Filter titles (parameterized query)", s.filter).Grow(1).Disabled(s.busy), ui.Button("sqlite/search-button", "Search", func() {
			d.sqliteAction(func(context.Context, *sqlite.DB) (string, error) { return "Loaded through Repository.List", nil })
		}).Variant(ui.Outline).Disabled(s.busy || !s.ready), ui.Button("sqlite/rollback", "Test rollback", func() {
			d.sqliteAction(func(ctx context.Context, db *sqlite.DB) (string, error) {
				rollback := errors.New("demo rollback")
				err := db.WithTransaction(ctx, func(tx *gorm.DB) error {
					if err := tx.Create(&sqliteTask{Title: "THIS ROW MUST ROLL BACK"}).Error; err != nil {
						return err
					}
					return rollback
				})
				if !errors.Is(err, rollback) {
					return "", err
				}
				return "Rollback verified: temporary row was not committed", nil
			})
		}).Variant(ui.Outline).Disabled(s.busy || !s.ready)).Gap(12),
		ui.Card("sqlite/results", ui.Text("sqlite/count", fmt.Sprintf("%d rows shown · maximum 100", len(s.rows))), ui.Column("sqlite/rows", rows...).Gap(8)),
		ui.Text("sqlite/status", s.message).Wrap(true),
		ui.Text("sqlite/code", "db, err := sqlite.Open(ctx, path, sqlite.Options{Models: []any{&Task{}}})\ntasks := sqlite.Table[Task](db)\nerr = tasks.Create(ctx, &Task{Title: \"Hello\"})\nrows, err := tasks.List(ctx)").Wrap(true),
		ui.Text("sqlite/lifetime", "Closing this window cancels work and closes its in-memory database.").Wrap(true),
	).Gap(16)
}
