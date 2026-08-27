// Package pg holds the pgslot CLI's database access -- thin wrappers around
// the pgslot.* views only (never the raw tables, matching pgslot's own
// storage/API split).
package pg

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

// Connect opens a connection using dsn, or -- if empty -- the standard
// PG* environment variables (PGHOST, PGUSER, PGDATABASE, ...), same as psql.
func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

type SlotHealth struct {
	Name          string
	Active        bool
	Status        string
	Reason        string
	RetainedBytes sql.NullInt64
	WalGrowth     sql.NullFloat64
	ConsumerRate  sql.NullFloat64
	LastSampleAt  time.Time
}

func FetchSlotHealth(db *sql.DB) ([]SlotHealth, error) {
	rows, err := db.Query(`
		SELECT slot_name, active, status, reason, retained_bytes,
		       wal_growth_bytes_per_sec, consumer_bytes_per_sec, last_sample_at
		FROM pgslot.slot_health
		ORDER BY slot_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SlotHealth
	for rows.Next() {
		var s SlotHealth
		if err := rows.Scan(&s.Name, &s.Active, &s.Status, &s.Reason,
			&s.RetainedBytes, &s.WalGrowth, &s.ConsumerRate, &s.LastSampleAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type WalSummary struct {
	SlotCount       int
	ActiveCount     int
	TotalRetained   sql.NullInt64
	MaxRetained     sql.NullInt64
	TopConsumerSlot sql.NullString
}

func FetchWalSummary(db *sql.DB) (WalSummary, error) {
	var s WalSummary
	err := db.QueryRow(`
		SELECT slot_count, active_count, total_retained_bytes,
		       max_retained_bytes, top_consumer_slot
		FROM pgslot.wal_summary`).
		Scan(&s.SlotCount, &s.ActiveCount, &s.TotalRetained, &s.MaxRetained, &s.TopConsumerSlot)
	return s, err
}

type HistoryPoint struct {
	CollectedAt   time.Time
	Active        bool
	WalStatus     sql.NullString
	RetainedBytes sql.NullInt64
	WalGrowth     sql.NullFloat64
	ConsumerRate  sql.NullFloat64
}

// FetchHistory returns up to limit snapshots for slotName, newest first.
func FetchHistory(db *sql.DB, slotName string, limit int) ([]HistoryPoint, error) {
	rows, err := db.Query(`
		SELECT collected_at, active, wal_status, retained_bytes,
		       wal_growth_bytes_per_sec, consumer_bytes_per_sec
		FROM pgslot.slot_history_rates
		WHERE slot_name = $1
		ORDER BY collected_at DESC
		LIMIT $2`, slotName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryPoint
	for rows.Next() {
		var h HistoryPoint
		if err := rows.Scan(&h.CollectedAt, &h.Active, &h.WalStatus, &h.RetainedBytes,
			&h.WalGrowth, &h.ConsumerRate); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
