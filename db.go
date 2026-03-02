package main

import (
	"database/sql"
	"strings"
	"time"

	"github.com/medeirosvictor/decent-notes/shared/types"
)

// Re-export shared types so the rest of the server package can use
// unqualified names (Entry, HabitData, EntryType, TypeNote, etc.).
type Entry = types.Entry
type HabitData = types.HabitData
type EntryType = types.EntryType

const (
	TypeNote  = types.TypeNote
	TypeTodo  = types.TypeTodo
	TypeHabit = types.TypeHabit
)

type DB struct {
	db *sql.DB
}

func NewDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS entries (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		is_done INTEGER DEFAULT 0,
		is_habit INTEGER DEFAULT 0,
		repeat_days TEXT DEFAULT '',
		repeat_hour TEXT DEFAULT '',
		times_completed INTEGER DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	_, err := db.db.Exec(schema)
	return err
}

func (db *DB) CreateEntry(e *Entry) error {
	repeatDays := ""
	repeatHour := ""
	timesCompleted := 0
	if e.HabitData != nil {
		repeatDays = strings.Join(e.HabitData.RepeatDays, ",")
		repeatHour = e.HabitData.RepeatHour
		timesCompleted = e.HabitData.TimesCompleted
	}

	sql := `
		INSERT INTO entries (id, type, title, description, is_done, is_habit, repeat_days, repeat_hour, times_completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.db.Exec(sql,
		e.ID, e.Type, e.Title, e.Description,
		boolToInt(e.IsDone), boolToInt(e.IsHabit),
		repeatDays, repeatHour, timesCompleted,
		e.CreatedAt.Format(time.RFC3339), e.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (db *DB) GetAllEntries() ([]Entry, error) {
	sql := "SELECT id, type, title, description, is_done, is_habit, repeat_days, repeat_hour, times_completed, created_at, updated_at FROM entries ORDER BY created_at DESC"
	rows, err := db.db.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var isDone, isHabit, timesCompleted int
		var repeatDays, repeatHour string
		var createdAt, updatedAt string

		err := rows.Scan(&e.ID, &e.Type, &e.Title, &e.Description, &isDone, &isHabit, &repeatDays, &repeatHour, &timesCompleted, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		e.IsDone = intToBool(isDone)
		e.IsHabit = intToBool(isHabit)
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		if e.IsHabit && repeatDays != "" {
			e.HabitData = &HabitData{
				RepeatDays:     strings.Split(repeatDays, ","),
				RepeatHour:     repeatHour,
				TimesCompleted: timesCompleted,
			}
		}

		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) GetEntry(id string) (*Entry, error) {
	sql := "SELECT id, type, title, description, is_done, is_habit, repeat_days, repeat_hour, times_completed, created_at, updated_at FROM entries WHERE id = ?"
	row := db.db.QueryRow(sql, id)

	var e Entry
	var isDone, isHabit, timesCompleted int
	var repeatDays, repeatHour string
	var createdAt, updatedAt string

	err := row.Scan(&e.ID, &e.Type, &e.Title, &e.Description, &isDone, &isHabit, &repeatDays, &repeatHour, &timesCompleted, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	e.IsDone = intToBool(isDone)
	e.IsHabit = intToBool(isHabit)
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if e.IsHabit && repeatDays != "" {
		e.HabitData = &HabitData{
			RepeatDays:     strings.Split(repeatDays, ","),
			RepeatHour:     repeatHour,
			TimesCompleted: timesCompleted,
		}
	}

	return &e, nil
}

func (db *DB) UpdateEntry(e *Entry) error {
	repeatDays := ""
	repeatHour := ""
	timesCompleted := 0
	if e.HabitData != nil {
		repeatDays = strings.Join(e.HabitData.RepeatDays, ",")
		repeatHour = e.HabitData.RepeatHour
		timesCompleted = e.HabitData.TimesCompleted
	}

	sql := `
		UPDATE entries SET type=?, title=?, description=?, is_done=?, is_habit=?, repeat_days=?, repeat_hour=?, times_completed=?, updated_at=? WHERE id=?
	`
	_, err := db.db.Exec(sql,
		e.Type, e.Title, e.Description,
		boolToInt(e.IsDone), boolToInt(e.IsHabit),
		repeatDays, repeatHour, timesCompleted,
		e.UpdatedAt.Format(time.RFC3339),
		e.ID,
	)
	return err
}

func (db *DB) DeleteEntry(id string) error {
	sql := "DELETE FROM entries WHERE id = ?"
	_, err := db.db.Exec(sql, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}

