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

type PipelineRow struct {
	SlotName        string
	Status          string
	Reason          string
	RetainedBytes   sql.NullInt64
	AdapterName     sql.NullString
	AdapterMetrics  []byte // raw jsonb, nil if no adapter has reported for this slot
	AdapterSampleAt sql.NullTime
	// Active is independent of Status: a slot can be critical while still
	// active, or critical because it is inactive -- the two are not the
	// same fact (added in slot_pipeline as of pgslot 0.5).
	Active bool
}

// FetchPipeline returns pgslot.slot_pipeline -- slot_health joined against
// each slot's latest adapter-reported metrics (Walkrie, or any other
// pgslot_adapter member). AdapterMetrics is schema-free by design (see
// adapters/README.md), so callers must not assume any particular key exists.
func FetchPipeline(db *sql.DB) ([]PipelineRow, error) {
	rows, err := db.Query(`
		SELECT slot_name, status, reason, retained_bytes,
		       adapter_name, adapter_metrics, adapter_sample_at, active
		FROM pgslot.slot_pipeline
		ORDER BY slot_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PipelineRow
	for rows.Next() {
		var p PipelineRow
		if err := rows.Scan(&p.SlotName, &p.Status, &p.Reason, &p.RetainedBytes,
			&p.AdapterName, &p.AdapterMetrics, &p.AdapterSampleAt, &p.Active); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type AvailablePublication struct {
	Name      string
	Owner     string
	AllTables bool
	Insert    bool
	Update    bool
	Delete    bool
	Truncate  bool
	ViaRoot   bool
}

// FetchAvailablePublications returns pgslot.available_publications --
// informational only, cannot be joined against slot_health since
// pg_replication_slots does not record which publication a slot consumes.
func FetchAvailablePublications(db *sql.DB) ([]AvailablePublication, error) {
	rows, err := db.Query(`
		SELECT pubname, owner, puballtables, pubinsert, pubupdate,
		       pubdelete, pubtruncate, pubviaroot
		FROM pgslot.available_publications
		ORDER BY pubname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AvailablePublication
	for rows.Next() {
		var p AvailablePublication
		if err := rows.Scan(&p.Name, &p.Owner, &p.AllTables, &p.Insert,
			&p.Update, &p.Delete, &p.Truncate, &p.ViaRoot); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type PipelineHistoryPoint struct {
	AdapterName string
	Metrics     []byte // raw jsonb
	CollectedAt time.Time
}

// FetchPipelineHistory returns pgslot.slot_pipeline_history for slotName --
// the full per-report time series behind slot_pipeline's latest-only join,
// same relationship FetchHistory/slot_history_rates has to slot_current.
// Runs on its own time axis: adapter report ticks and pgslot collect()
// ticks are independently scheduled, so don't zip these rows against
// FetchHistory's by index or timestamp.
func FetchPipelineHistory(db *sql.DB, slotName string, limit int) ([]PipelineHistoryPoint, error) {
	rows, err := db.Query(`
		SELECT adapter_name, metrics, collected_at
		FROM pgslot.slot_pipeline_history
		WHERE slot_name = $1
		ORDER BY collected_at DESC
		LIMIT $2`, slotName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PipelineHistoryPoint
	for rows.Next() {
		var p PipelineHistoryPoint
		if err := rows.Scan(&p.AdapterName, &p.Metrics, &p.CollectedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
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
