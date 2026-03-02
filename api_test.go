package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

// ── Helper ────────────────────────────────────────────────

// newTestMux creates a ServeMux wired to a fresh in-memory DB.
func newTestMux(t *testing.T) (*http.ServeMux, *DB) {
	t.Helper()
	db := newTestDB(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/entries", entriesHandler(db))
	mux.HandleFunc("/entries/{id}", entryHandler(db))
	return mux, db
}

// doRequest is a shorthand for building and executing a test request.
func doRequest(mux http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// decodeEntry decodes the response body into an Entry.
func decodeEntry(t *testing.T, rec *httptest.ResponseRecorder) Entry {
	t.Helper()
	var entry Entry
	if err := json.NewDecoder(rec.Body).Decode(&entry); err != nil {
		t.Fatalf("Failed to decode response: %v\nBody: %s", err, rec.Body.String())
	}
	return entry
}

// decodeEntries decodes the response body into a slice of Entry.
func decodeEntries(t *testing.T, rec *httptest.ResponseRecorder) []Entry {
	t.Helper()
	var entries []Entry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("Failed to decode response: %v\nBody: %s", err, rec.Body.String())
	}
	return entries
}

// ── Health ────────────────────────────────────────────────

func TestHealthEndpoint(t *testing.T) {
	mux, _ := newTestMux(t)
	rec := doRequest(mux, "GET", "/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
	if body["time"] == "" {
		t.Error("expected time field to be set")
	}
}

// ── Create Entry ──────────────────────────────────────────

func TestCreateEntry_Note(t *testing.T) {
	mux, _ := newTestMux(t)
	payload := map[string]any{
		"type":        "note",
		"title":       "Test Note",
		"description": "Some content",
	}

	rec := doRequest(mux, "POST", "/entries", payload)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	entry := decodeEntry(t, rec)
	if entry.ID == "" {
		t.Error("expected ID to be generated")
	}
	if entry.Title != "Test Note" {
		t.Errorf("expected title 'Test Note', got %q", entry.Title)
	}
	if entry.Description != "Some content" {
		t.Errorf("expected description 'Some content', got %q", entry.Description)
	}
	if entry.Type != "note" {
		t.Errorf("expected type 'note', got %q", entry.Type)
	}
}

func TestCreateEntry_Todo(t *testing.T) {
	mux, _ := newTestMux(t)
	payload := map[string]any{
		"type":  "todo",
		"title": "Buy groceries",
	}

	rec := doRequest(mux, "POST", "/entries", payload)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	entry := decodeEntry(t, rec)
	if entry.Title != "Buy groceries" {
		t.Errorf("expected title 'Buy groceries', got %q", entry.Title)
	}
	if entry.IsDone {
		t.Error("expected isDone to be false for new todo")
	}
}

func TestCreateEntry_InvalidJSON(t *testing.T) {
	mux, _ := newTestMux(t)
	req := httptest.NewRequest("POST", "/entries", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateEntry_GeneratesID(t *testing.T) {
	mux, _ := newTestMux(t)
	// No ID provided — server should generate one
	payload := map[string]any{"type": "note", "title": "Auto ID"}

	rec := doRequest(mux, "POST", "/entries", payload)
	entry := decodeEntry(t, rec)

	if entry.ID == "" {
		t.Error("expected server to generate an ID")
	}
}

func TestCreateEntry_SetsTimestamps(t *testing.T) {
	mux, _ := newTestMux(t)
	payload := map[string]any{"type": "note", "title": "Timestamps"}

	rec := doRequest(mux, "POST", "/entries", payload)
	entry := decodeEntry(t, rec)

	if entry.CreatedAt.IsZero() {
		t.Error("expected createdAt to be set")
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("expected updatedAt to be set")
	}
}

// ── Get Entries ───────────────────────────────────────────

func TestGetEntries_Empty(t *testing.T) {
	mux, _ := newTestMux(t)
	rec := doRequest(mux, "GET", "/entries", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Empty DB returns null (json.Encode of nil slice)
	body := rec.Body.String()
	if body != "null\n" {
		entries := decodeEntries(t, rec)
		if len(entries) != 0 {
			t.Errorf("expected empty list, got %d entries", len(entries))
		}
	}
}

func TestGetEntries_ReturnsList(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create two entries
	doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "First"})
	doRequest(mux, "POST", "/entries", map[string]any{"type": "todo", "title": "Second"})

	rec := doRequest(mux, "GET", "/entries", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	entries := decodeEntries(t, rec)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

// ── Get Single Entry ──────────────────────────────────────

func TestGetEntry_Found(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create an entry
	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Fetch Me"})
	created := decodeEntry(t, createRec)

	// Fetch it
	rec := doRequest(mux, "GET", "/entries/"+created.ID, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	entry := decodeEntry(t, rec)
	if entry.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, entry.ID)
	}
	if entry.Title != "Fetch Me" {
		t.Errorf("expected title 'Fetch Me', got %q", entry.Title)
	}
}

func TestGetEntryAPI_NotFound(t *testing.T) {
	mux, _ := newTestMux(t)
	rec := doRequest(mux, "GET", "/entries/nonexistent-id", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ── Update Entry ──────────────────────────────────────────

func TestUpdateEntry_Title(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create
	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Original"})
	created := decodeEntry(t, createRec)

	// Update
	rec := doRequest(mux, "PUT", "/entries/"+created.ID, map[string]any{"title": "Updated"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	entry := decodeEntry(t, rec)
	if entry.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", entry.Title)
	}

	// Verify via GET
	getRec := doRequest(mux, "GET", "/entries/"+created.ID, nil)
	fetched := decodeEntry(t, getRec)
	if fetched.Title != "Updated" {
		t.Errorf("GET after update: expected 'Updated', got %q", fetched.Title)
	}
}

func TestUpdateEntry_ToggleDone(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create a todo
	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "todo", "title": "Toggle me"})
	created := decodeEntry(t, createRec)

	if created.IsDone {
		t.Fatal("expected isDone=false initially")
	}

	// Toggle to done
	rec := doRequest(mux, "PUT", "/entries/"+created.ID, map[string]any{"isDone": true})
	updated := decodeEntry(t, rec)

	if !updated.IsDone {
		t.Error("expected isDone=true after update")
	}
}

func TestUpdateEntry_Description(t *testing.T) {
	mux, _ := newTestMux(t)

	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Note", "description": ""})
	created := decodeEntry(t, createRec)

	rec := doRequest(mux, "PUT", "/entries/"+created.ID, map[string]any{"description": "Added content"})
	updated := decodeEntry(t, rec)

	if updated.Description != "Added content" {
		t.Errorf("expected description 'Added content', got %q", updated.Description)
	}
}

func TestUpdateEntry_InvalidJSON(t *testing.T) {
	mux, _ := newTestMux(t)

	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Note"})
	created := decodeEntry(t, createRec)

	req := httptest.NewRequest("PUT", "/entries/"+created.ID, bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ── Delete Entry ──────────────────────────────────────────

func TestDeleteEntry_Success(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create
	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Delete me"})
	created := decodeEntry(t, createRec)

	// Delete
	rec := doRequest(mux, "DELETE", "/entries/"+created.ID, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify it's gone
	getRec := doRequest(mux, "GET", "/entries/"+created.ID, nil)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getRec.Code)
	}
}

func TestDeleteEntry_RemovedFromList(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create two
	doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Keep"})
	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Remove"})
	toDelete := decodeEntry(t, createRec)

	// Delete one
	doRequest(mux, "DELETE", "/entries/"+toDelete.ID, nil)

	// List should have one
	listRec := doRequest(mux, "GET", "/entries", nil)
	entries := decodeEntries(t, listRec)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", len(entries))
	}
	if entries[0].Title != "Keep" {
		t.Errorf("expected remaining entry 'Keep', got %q", entries[0].Title)
	}
}

// ── Method Not Allowed ────────────────────────────────────

func TestEntries_MethodNotAllowed(t *testing.T) {
	mux, _ := newTestMux(t)
	rec := doRequest(mux, "PATCH", "/entries", nil)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestEntry_MethodNotAllowed(t *testing.T) {
	mux, _ := newTestMux(t)

	createRec := doRequest(mux, "POST", "/entries", map[string]any{"type": "note", "title": "Test"})
	created := decodeEntry(t, createRec)

	rec := doRequest(mux, "PATCH", "/entries/"+created.ID, nil)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ── Entry Type Enforcement ─────────────────────────────────

func TestCreateTodo_StripsDescription(t *testing.T) {
	mux, _ := newTestMux(t)
	payload := map[string]any{
		"type":        "todo",
		"title":       "Buy milk",
		"description": "should be stripped",
		"isHabit":     true,
	}

	rec := doRequest(mux, "POST", "/entries", payload)
	entry := decodeEntry(t, rec)

	if entry.Description != "" {
		t.Errorf("expected empty description for todo, got %q", entry.Description)
	}
	if entry.IsHabit {
		t.Error("expected isHabit=false for todo")
	}
}

func TestCreateNote_KeepsDescription(t *testing.T) {
	mux, _ := newTestMux(t)
	payload := map[string]any{
		"type":        "note",
		"title":       "My note",
		"description": "some content",
	}

	rec := doRequest(mux, "POST", "/entries", payload)
	entry := decodeEntry(t, rec)

	if entry.Description != "some content" {
		t.Errorf("expected description 'some content', got %q", entry.Description)
	}
}

// ── Full CRUD Round-Trip ──────────────────────────────────

func TestFullCRUDRoundTrip(t *testing.T) {
	mux, _ := newTestMux(t)

	// 1. Create
	createRec := doRequest(mux, "POST", "/entries", map[string]any{
		"type":        "note",
		"title":       "My Note",
		"description": "Initial content",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("Create: expected 201, got %d", createRec.Code)
	}
	created := decodeEntry(t, createRec)

	// 2. Read
	getRec := doRequest(mux, "GET", "/entries/"+created.ID, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("Read: expected 200, got %d", getRec.Code)
	}
	fetched := decodeEntry(t, getRec)
	if fetched.Title != "My Note" {
		t.Errorf("Read: expected 'My Note', got %q", fetched.Title)
	}

	// 3. Update
	updateRec := doRequest(mux, "PUT", "/entries/"+created.ID, map[string]any{
		"title":       "Updated Note",
		"description": "New content",
	})
	if updateRec.Code != http.StatusOK {
		t.Fatalf("Update: expected 200, got %d", updateRec.Code)
	}
	updated := decodeEntry(t, updateRec)
	if updated.Title != "Updated Note" {
		t.Errorf("Update: expected 'Updated Note', got %q", updated.Title)
	}
	if updated.Description != "New content" {
		t.Errorf("Update: expected 'New content', got %q", updated.Description)
	}

	// 4. Delete
	deleteRec := doRequest(mux, "DELETE", "/entries/"+created.ID, nil)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("Delete: expected 204, got %d", deleteRec.Code)
	}

	// 5. Verify gone
	goneRec := doRequest(mux, "GET", "/entries/"+created.ID, nil)
	if goneRec.Code != http.StatusNotFound {
		t.Errorf("After delete: expected 404, got %d", goneRec.Code)
	}
}
