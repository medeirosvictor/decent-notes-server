// Package types defines the shared data model for Decent Notes.
// Both the server and CLI import this package so there is a single
// source of truth for JSON shapes exchanged over the REST API.
package types

import "time"

// EntryType constrains the allowed values for Entry.Type.
type EntryType string

const (
	TypeNote  EntryType = "note"
	TypeTodo  EntryType = "todo"
	TypeHabit EntryType = "habit"
)

// Entry is the unified data model. Notes, todos, and habits are all
// stored as entries with a discriminating Type field.
type Entry struct {
	ID          string     `json:"id"`
	Type        EntryType  `json:"type"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	IsDone      bool       `json:"isDone,omitempty"`
	IsHabit     bool       `json:"isHabit,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	HabitData   *HabitData `json:"habitData,omitempty"`
}

// HabitData holds the recurrence configuration for habit entries.
type HabitData struct {
	RepeatDays     []string `json:"repeatDays,omitempty"`
	RepeatHour     string   `json:"repeatHour,omitempty"`
	TimesCompleted int      `json:"timesCompleted,omitempty"`
}

// ApiResponse is the generic wrapper returned by the REST API.
type ApiResponse[T any] struct {
	Data    T      `json:"data"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// HealthResponse is the shape returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}
