package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

// NewStorage creates a new SQLite storage instance
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{db: db}
	if err := storage.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return storage, nil
}

// initialize creates the database schema
func (s *Storage) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS change_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		namespace TEXT NOT NULL,
		kind TEXT NOT NULL,
		name TEXT NOT NULL,
		action TEXT NOT NULL,
		diff TEXT,
		metadata TEXT,
		image_before TEXT,
		image_after TEXT
	);
	
	CREATE INDEX IF NOT EXISTS idx_timestamp ON change_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_namespace ON change_events(namespace);
	CREATE INDEX IF NOT EXISTS idx_kind ON change_events(kind);
	CREATE INDEX IF NOT EXISTS idx_name ON change_events(name);
	CREATE INDEX IF NOT EXISTS idx_action ON change_events(action);
	
	-- Composite indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_namespace_kind_name ON change_events(namespace, kind, name);
	CREATE INDEX IF NOT EXISTS idx_kind_timestamp ON change_events(kind, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_namespace_timestamp ON change_events(namespace, timestamp DESC);
	`
	_, err := s.db.Exec(schema)
	return err
}

// CleanupOldEvents removes events older than the specified number of days
func (s *Storage) CleanupOldEvents(retentionDays int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	result, err := s.db.Exec("DELETE FROM change_events WHERE timestamp < ?", cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old events: %w", err)
	}
	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// GetTotalCount returns total count of events matching filter
func (s *Storage) GetTotalCount(filter Filter) (int64, error) {
	query := `SELECT COUNT(*) FROM change_events WHERE 1=1`
	args := []interface{}{}

	if filter.Namespace != "" {
		query += " AND namespace = ?"
		args = append(args, filter.Namespace)
	}
	if filter.Kind != "" {
		query += " AND kind = ?"
		args = append(args, filter.Kind)
	}
	if filter.Name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if !filter.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime)
	}

	var count int64
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// SaveEvent saves a change event to the database
func (s *Storage) SaveEvent(event *ChangeEvent) error {
	query := `
		INSERT INTO change_events (timestamp, namespace, kind, name, action, diff, metadata, image_before, image_after)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.Exec(query,
		event.Timestamp,
		event.Namespace,
		event.Kind,
		event.Name,
		event.Action,
		event.Diff,
		event.Metadata,
		event.ImageBefore,
		event.ImageAfter,
	)
	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		event.ID = id
	}

	return nil
}

// GetEvents retrieves events with filters
func (s *Storage) GetEvents(filter Filter) ([]ChangeEvent, error) {
	query := `SELECT id, timestamp, namespace, kind, name, action, diff, metadata, image_before, image_after
	          FROM change_events WHERE 1=1`
	args := []interface{}{}

	if filter.Namespace != "" {
		query += " AND namespace = ?"
		args = append(args, filter.Namespace)
	}
	if filter.Kind != "" {
		query += " AND kind = ?"
		args = append(args, filter.Kind)
	}
	if filter.Name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if !filter.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []ChangeEvent
	for rows.Next() {
		var event ChangeEvent
		var imageBefore, imageAfter sql.NullString
		err := rows.Scan(
			&event.ID,
			&event.Timestamp,
			&event.Namespace,
			&event.Kind,
			&event.Name,
			&event.Action,
			&event.Diff,
			&event.Metadata,
			&imageBefore,
			&imageAfter,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if imageBefore.Valid {
			event.ImageBefore = imageBefore.String
		}
		if imageAfter.Valid {
			event.ImageAfter = imageAfter.String
		}
		events = append(events, event)
	}

	return events, nil
}

// GetStats retrieves dashboard statistics
func (s *Storage) GetStats() (*Stats, error) {
	stats := &Stats{
		ChangesByKind:   make(map[string]int64),
		ChangesByAction: make(map[string]int64),
	}

	// Total changes
	err := s.db.QueryRow("SELECT COUNT(*) FROM change_events").Scan(&stats.TotalChanges)
	if err != nil {
		return nil, err
	}

	// Changes in last 24h
	last24h := time.Now().Add(-24 * time.Hour)
	err = s.db.QueryRow("SELECT COUNT(*) FROM change_events WHERE timestamp >= ?", last24h).Scan(&stats.ChangesLast24h)
	if err != nil {
		return nil, err
	}

	stats.ChangesPerHour = float64(stats.ChangesLast24h) / 24.0

	// Top modified apps
	rows, err := s.db.Query(`
		SELECT name, COUNT(*) as count 
		FROM change_events 
		WHERE timestamp >= ? 
		GROUP BY name 
		ORDER BY count DESC 
		LIMIT 10
	`, last24h)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var app AppChangeCount
		rows.Scan(&app.Name, &app.Count)
		stats.TopModifiedApps = append(stats.TopModifiedApps, app)
	}

	// Recent images
	imageRows, err := s.db.Query(`
		SELECT DISTINCT image_after 
		FROM change_events 
		WHERE image_after IS NOT NULL AND image_after != '' 
		ORDER BY timestamp DESC 
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer imageRows.Close()
	for imageRows.Next() {
		var image string
		imageRows.Scan(&image)
		stats.RecentImages = append(stats.RecentImages, image)
	}

	// Changes by kind
	kindRows, err := s.db.Query("SELECT kind, COUNT(*) FROM change_events GROUP BY kind")
	if err != nil {
		return nil, err
	}
	defer kindRows.Close()
	for kindRows.Next() {
		var kind string
		var count int64
		kindRows.Scan(&kind, &count)
		stats.ChangesByKind[kind] = count
	}

	// Changes by action
	actionRows, err := s.db.Query("SELECT action, COUNT(*) FROM change_events GROUP BY action")
	if err != nil {
		return nil, err
	}
	defer actionRows.Close()
	for actionRows.Next() {
		var action string
		var count int64
		actionRows.Scan(&action, &count)
		stats.ChangesByAction[action] = count
	}

	return stats, nil
}

// GetTimeline retrieves timeline for a specific resource
func (s *Storage) GetTimeline(namespace, kind, name string) ([]ChangeEvent, error) {
	query := `
		SELECT id, timestamp, namespace, kind, name, action, diff, metadata, image_before, image_after
		FROM change_events 
		WHERE namespace = ? AND kind = ? AND name = ?
		ORDER BY timestamp DESC
	`
	rows, err := s.db.Query(query, namespace, kind, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer rows.Close()

	var events []ChangeEvent
	for rows.Next() {
		var event ChangeEvent
		var imageBefore, imageAfter sql.NullString
		err := rows.Scan(
			&event.ID,
			&event.Timestamp,
			&event.Namespace,
			&event.Kind,
			&event.Name,
			&event.Action,
			&event.Diff,
			&event.Metadata,
			&imageBefore,
			&imageAfter,
		)
		if err != nil {
			return nil, err
		}
		if imageBefore.Valid {
			event.ImageBefore = imageBefore.String
		}
		if imageAfter.Valid {
			event.ImageAfter = imageAfter.String
		}
		events = append(events, event)
	}

	return events, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}
