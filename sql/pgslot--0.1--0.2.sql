-- pgslot--0.1--0.2.sql
-- No schema changes -- 0.2 is a version-label bump only, marking the
-- phase-2 background worker (src/pgslot_worker.c) and the CLI (cli/) as
-- part of pgslot's current status rather than an unbuilt "phase 2" sketch.
-- If you already applied the 0.1 -> 0.2 view changes (slot_history_rates,
-- rebuilt slot_rates/slot_health/slot_pipeline) by hand, this file is a
-- no-op for you.

\echo Use "ALTER EXTENSION pgslot UPDATE TO '0.2'" to apply this upgrade. \quit
