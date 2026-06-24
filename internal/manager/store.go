package manager

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type StateStore interface {
	Init() error
	Load() ([]DownloadRecord, []string, error)
	SaveAll(jobs map[string]*managedDownload, queue []string) error
	Close() error
}

type DBStore struct {
	db *sql.DB
}

func NewDBStore(dbPath string) (*DBStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	store := &DBStore{db: db}
	if err := store.Init(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *DBStore) Init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS downloads (
		id TEXT PRIMARY KEY,
		url TEXT,
		destination TEXT,
		status TEXT,
		progress_downloaded INTEGER,
		progress_total INTEGER,
		progress_speed_bps REAL,
		progress_eta INTEGER,
		progress_workers INTEGER,
		progress_retries INTEGER,
		options_json TEXT,
		error TEXT,
		started_at DATETIME,
		completed_at DATETIME,
		active_for INTEGER,
		created_at DATETIME,
		updated_at DATETIME,
		queue_order INTEGER
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *DBStore) Load() ([]DownloadRecord, []string, error) {
	query := `
	SELECT 
		id, url, destination, status, 
		progress_downloaded, progress_total, progress_speed_bps, progress_eta, progress_workers, progress_retries,
		options_json,
		error, started_at, completed_at, active_for, created_at, updated_at, queue_order
	FROM downloads
	ORDER BY queue_order ASC;
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var records []DownloadRecord
	var queue []string

	for rows.Next() {
		var rec DownloadRecord
		var queueOrder int
		var optionsJSON string
		var startedAt, completedAt, createdAt, updatedAt sql.NullTime

		err := rows.Scan(
			&rec.ID, &rec.URL, &rec.Destination, &rec.Status,
			&rec.Progress.Downloaded, &rec.Progress.Total, &rec.Progress.SpeedBps, &rec.Progress.ETA, &rec.Progress.Workers, &rec.Progress.Retries,
			&optionsJSON,
			&rec.Error, &startedAt, &completedAt, &rec.ActiveFor, &createdAt, &updatedAt, &queueOrder,
		)
		if err != nil {
			return nil, nil, err
		}

		if startedAt.Valid {
			rec.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			rec.CompletedAt = completedAt.Time
		}
		if createdAt.Valid {
			rec.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			rec.UpdatedAt = updatedAt.Time
		}

		if optionsJSON != "" {
			_ = json.Unmarshal([]byte(optionsJSON), &rec.Options)
		}

		records = append(records, rec)
		if queueOrder >= 0 && rec.Status == StatusQueued {
			queue = append(queue, rec.ID)
		}
	}
	
	return records, queue, nil
}

func (s *DBStore) SaveAll(jobs map[string]*managedDownload, queue []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM downloads")
	if err != nil {
		tx.Rollback()
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO downloads (
			id, url, destination, status, 
			progress_downloaded, progress_total, progress_speed_bps, progress_eta, progress_workers, progress_retries,
			options_json, 
			error, started_at, completed_at, active_for, created_at, updated_at, queue_order
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	queueIndexes := make(map[string]int)
	for i, id := range queue {
		queueIndexes[id] = i
	}

	for _, job := range jobs {
		rec := job.rec
		qOrder := -1
		if idx, ok := queueIndexes[rec.ID]; ok {
			qOrder = idx
		}

		optionsBytes, _ := json.Marshal(rec.Options)

		_, err = stmt.Exec(
			rec.ID, rec.URL, rec.Destination, string(rec.Status),
			rec.Progress.Downloaded, rec.Progress.Total, rec.Progress.SpeedBps, rec.Progress.ETA, rec.Progress.Workers, rec.Progress.Retries,
			string(optionsBytes),
			rec.Error,
			sqlNullTime(rec.StartedAt), sqlNullTime(rec.CompletedAt), rec.ActiveFor, sqlNullTime(rec.CreatedAt), sqlNullTime(rec.UpdatedAt),
			qOrder,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (s *DBStore) Close() error {
	return s.db.Close()
}

func sqlNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}

