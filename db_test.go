package main

import (
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── Helper ────────────────────────────────────────────────
// newTestDB creates a fresh in-memory SQLite database with the schema applied.
// Each test gets its own DB so they never interfere with each other.
func newTestDB(t *testing.T) *DB {
	t.Helper() // marks this as a helper — errors report the caller's line, not this one

	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB(:memory:) failed: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// t.Cleanup registers a function that runs when this test finishes,
	// even if the test fails. Like defer but tied to the test lifecycle.
	t.Cleanup(func() { db.Close() })

	return db
}

// ── Create + Read ─────────────────────────────────────────

func TestCreateAndGetNote(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().Truncate(time.Second) // Truncate because RFC3339 drops sub-second

	entry := &Entry{
		ID:          "note-1",
		Type:        TypeNote,
		Title:       "My First Note",
		Description: "Some content here",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create
	if err := db.CreateEntry(entry); err != nil {
		t.Fatalf("CreateEntry() error: %v", err)
	}

	// Read it back
	got, err := db.GetEntry("note-1")
	if err != nil {
		t.Fatalf("GetEntry() error: %v", err)
	}

	// Verify each field
	if got.ID != "note-1" {
		t.Errorf("ID = %q, want %q", got.ID, "note-1")
	}
	if got.Type != TypeNote {
		t.Errorf("Type = %q, want %q", got.Type, TypeNote)
	}
	if got.Title != "My First Note" {
		t.Errorf("Title = %q, want %q", got.Title, "My First Note")
	}
	if got.Description != "Some content here" {
		t.Errorf("Description = %q, want %q", got.Description, "Some content here")
	}
	if got.IsDone != false {
		t.Errorf("IsDone = %v, want false", got.IsDone)
	}
	if got.HabitData != nil {
		t.Errorf("HabitData = %+v, want nil (notes shouldn't have habit data)", got.HabitData)
	}
}

func TestCreateAndGetTodo(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	entry := &Entry{
		ID:        "todo-1",
		Type:      TypeTodo,
		Title:     "Buy groceries",
		IsDone:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.CreateEntry(entry); err != nil {
		t.Fatalf("CreateEntry() error: %v", err)
	}

	got, err := db.GetEntry("todo-1")
	if err != nil {
		t.Fatalf("GetEntry() error: %v", err)
	}

	if got.IsDone != true {
		t.Errorf("IsDone = %v, want true", got.IsDone)
	}
	if got.Title != "Buy groceries" {
		t.Errorf("Title = %q, want %q", got.Title, "Buy groceries")
	}
}

func TestCreateAndGetHabit(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	entry := &Entry{
		ID:      "habit-1",
		Type:    TypeHabit,
		Title:   "Morning run",
		IsHabit: true,
		HabitData: &HabitData{
			RepeatDays:     []string{"monday", "wednesday", "friday"},
			RepeatHour:     "07:00",
			TimesCompleted: 5,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.CreateEntry(entry); err != nil {
		t.Fatalf("CreateEntry() error: %v", err)
	}

	got, err := db.GetEntry("habit-1")
	if err != nil {
		t.Fatalf("GetEntry() error: %v", err)
	}

	// Habit data should survive the round-trip through the DB
	if got.HabitData == nil {
		t.Fatal("HabitData is nil, want non-nil")
	}
	if len(got.HabitData.RepeatDays) != 3 {
		t.Fatalf("RepeatDays length = %d, want 3", len(got.HabitData.RepeatDays))
	}
	if got.HabitData.RepeatDays[0] != "monday" {
		t.Errorf("RepeatDays[0] = %q, want %q", got.HabitData.RepeatDays[0], "monday")
	}
	if got.HabitData.RepeatDays[2] != "friday" {
		t.Errorf("RepeatDays[2] = %q, want %q", got.HabitData.RepeatDays[2], "friday")
	}
	if got.HabitData.RepeatHour != "07:00" {
		t.Errorf("RepeatHour = %q, want %q", got.HabitData.RepeatHour, "07:00")
	}
	if got.HabitData.TimesCompleted != 5 {
		t.Errorf("TimesCompleted = %d, want 5", got.HabitData.TimesCompleted)
	}
}

// ── GetEntry: missing ID ──────────────────────────────────

func TestGetEntry_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.GetEntry("does-not-exist")
	if err == nil {
		t.Fatal("GetEntry() returned nil error for missing ID, want an error")
	}
	// We don't check the exact error message — just that it fails.
	// This is intentional: we care about behavior, not implementation wording.
}

// ── GetAllEntries ─────────────────────────────────────────

func TestGetAllEntries_Empty(t *testing.T) {
	db := newTestDB(t)

	entries, err := db.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() error: %v", err)
	}
	// A fresh DB should return nil (not an empty slice) because append on nil returns nil.
	// Either nil or empty is acceptable — both have len() == 0.
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestGetAllEntries_ReturnsNewestFirst(t *testing.T) {
	db := newTestDB(t)

	// Create two entries with different timestamps
	older := &Entry{ID: "old", Type: TypeNote, Title: "Old note", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Now()}
	newer := &Entry{ID: "new", Type: TypeNote, Title: "New note", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Now()}

	// Insert old first, then new
	db.CreateEntry(older)
	db.CreateEntry(newer)

	entries, err := db.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	// ORDER BY created_at DESC — newest first
	if entries[0].ID != "new" {
		t.Errorf("entries[0].ID = %q, want %q (newest first)", entries[0].ID, "new")
	}
	if entries[1].ID != "old" {
		t.Errorf("entries[1].ID = %q, want %q", entries[1].ID, "old")
	}
}

// ── Update ────────────────────────────────────────────────

func TestUpdateEntry(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Create a todo
	entry := &Entry{ID: "todo-u", Type: TypeTodo, Title: "Original", CreatedAt: now, UpdatedAt: now}
	db.CreateEntry(entry)

	// Update it
	entry.Title = "Updated title"
	entry.IsDone = true
	entry.UpdatedAt = now.Add(time.Hour)
	if err := db.UpdateEntry(entry); err != nil {
		t.Fatalf("UpdateEntry() error: %v", err)
	}

	// Read it back
	got, _ := db.GetEntry("todo-u")
	if got.Title != "Updated title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated title")
	}
	if got.IsDone != true {
		t.Errorf("IsDone = %v, want true", got.IsDone)
	}
}

func TestUpdateEntry_HabitData(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Create a habit with some data
	entry := &Entry{
		ID: "habit-u", Type: TypeHabit, Title: "Exercise", IsHabit: true,
		HabitData: &HabitData{RepeatDays: []string{"monday"}, RepeatHour: "06:00", TimesCompleted: 1},
		CreatedAt: now, UpdatedAt: now,
	}
	db.CreateEntry(entry)

	// Update the habit data
	entry.HabitData = &HabitData{
		RepeatDays:     []string{"monday", "tuesday", "thursday"},
		RepeatHour:     "08:00",
		TimesCompleted: 10,
	}
	entry.UpdatedAt = now.Add(time.Hour)
	db.UpdateEntry(entry)

	got, _ := db.GetEntry("habit-u")
	if got.HabitData == nil {
		t.Fatal("HabitData is nil after update")
	}
	if len(got.HabitData.RepeatDays) != 3 {
		t.Errorf("RepeatDays length = %d, want 3", len(got.HabitData.RepeatDays))
	}
	if got.HabitData.RepeatHour != "08:00" {
		t.Errorf("RepeatHour = %q, want %q", got.HabitData.RepeatHour, "08:00")
	}
	if got.HabitData.TimesCompleted != 10 {
		t.Errorf("TimesCompleted = %d, want 10", got.HabitData.TimesCompleted)
	}
}

// ── Delete ────────────────────────────────────────────────

func TestDeleteEntry(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	entry := &Entry{ID: "del-1", Type: TypeNote, Title: "To be deleted", CreatedAt: now, UpdatedAt: now}
	db.CreateEntry(entry)

	// Confirm it exists
	if _, err := db.GetEntry("del-1"); err != nil {
		t.Fatalf("entry should exist before delete: %v", err)
	}

	// Delete it
	if err := db.DeleteEntry("del-1"); err != nil {
		t.Fatalf("DeleteEntry() error: %v", err)
	}

	// Confirm it's gone
	_, err := db.GetEntry("del-1")
	if err == nil {
		t.Fatal("GetEntry() returned nil error after delete, want error")
	}
}

func TestDeleteEntry_NonExistent(t *testing.T) {
	db := newTestDB(t)

	// Deleting something that doesn't exist should NOT return an error.
	// SQL DELETE with a WHERE that matches 0 rows is not an error — it's just 0 rows affected.
	err := db.DeleteEntry("ghost-id")
	if err != nil {
		t.Errorf("DeleteEntry(non-existent) error: %v, want nil", err)
	}
}

// ── Bug canary: CreateEntry ignores repeatHour and timesCompleted ──

func TestCreateEntry_HabitDataFieldsPreserved(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	entry := &Entry{
		ID: "habit-fields", Type: TypeHabit, Title: "Read", IsHabit: true,
		HabitData: &HabitData{
			RepeatDays:     []string{"saturday", "sunday"},
			RepeatHour:     "09:30",
			TimesCompleted: 42,
		},
		CreatedAt: now, UpdatedAt: now,
	}
	db.CreateEntry(entry)

	got, _ := db.GetEntry("habit-fields")
	if got.HabitData == nil {
		t.Fatal("HabitData is nil")
	}

	// BUG: CreateEntry hardcodes repeatHour="" and timesCompleted=0
	// instead of using the values from the input. These assertions
	// document the expected behavior. If they fail, it means CreateEntry
	// is still dropping these fields.
	if got.HabitData.RepeatHour != "09:30" {
		t.Errorf("RepeatHour = %q, want %q (CreateEntry may be ignoring this field)", got.HabitData.RepeatHour, "09:30")
	}
	if got.HabitData.TimesCompleted != 42 {
		t.Errorf("TimesCompleted = %d, want 42 (CreateEntry may be ignoring this field)", got.HabitData.TimesCompleted, )
	}
}

// ── Duplicate ID ──────────────────────────────────────────

func TestCreateEntry_DuplicateID(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().Truncate(time.Second)

	entry := &Entry{ID: "dup", Type: TypeNote, Title: "First", CreatedAt: now, UpdatedAt: now}
	if err := db.CreateEntry(entry); err != nil {
		t.Fatalf("first CreateEntry() error: %v", err)
	}

	// Inserting the same ID again should fail (PRIMARY KEY constraint)
	entry2 := &Entry{ID: "dup", Type: TypeNote, Title: "Second", CreatedAt: now, UpdatedAt: now}
	err := db.CreateEntry(entry2)
	if err == nil {
		t.Fatal("CreateEntry() with duplicate ID returned nil error, want constraint error")
	}
}
