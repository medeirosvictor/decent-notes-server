# Decent Notes Server

Go REST API server with SQLite storage.

## Quick Start

### Running Locally

```bash
cd decent-notes-server
go run .
```

Server runs on `http://localhost:5050`

### Running with Docker

```bash
docker compose up -d      # Start server
docker compose down      # Stop server
```

Server runs on `http://localhost:5050`

## Tech Stack

| Tool | Version |
|------|---------|
| Go | 1.25 |
| SQLite | via `modernc.org/sqlite` (pure Go, no CGO) |

## Commands

```bash
go run .              # run the server (default port 5050)
go build .            # build binary
go test ./...         # run all tests
go test -v ./...      # verbose test output
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `5050` | Server listen port |
| `DB_FILE` | `decent-notes.db` | SQLite file path |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/entries` | List all entries |
| POST | `/entries` | Create entry |
| GET | `/entries/{id}` | Get single entry |
| PUT | `/entries/{id}` | Update entry |
| DELETE | `/entries/{id}` | Delete entry |

## Data Model

```go
type Entry struct {
    ID          string
    Type        EntryType  // "note", "todo", or "habit"
    Title       string
    Description string
    IsDone      bool
    IsHabit     bool
    HabitData   *HabitData
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

## Connecting Clients

Any companion app can connect to the server:

| Client | Run Command |
|--------|-------------|
| **Web app** | `cd decent-notes-webapp && pnpm run dev` |
| **Desktop** | Run `decent-notes-desktop.exe` |
| **CLI** | `cd decent-notes-tui && go run .` |

All connect to `http://localhost:5050` by default.
