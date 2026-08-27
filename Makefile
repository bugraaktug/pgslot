EXTENSION   = pgslot
DATA        = sql/pgslot--0.1.sql
PGFILEDESC  = "pgslot - replication slot WAL health, history, and diagnostics"

# v0.1 ships no C sources -- collection is a plain SQL/PLpgSQL function
# called externally (systemd timer, k8s CronJob, manual psql, pgslot CLI).
# src/ is reserved for the phase-2 background worker (pgslot_worker.so),
# which will require shared_preload_libraries and a MODULES line here.

PG_CONFIG ?= pg_config
PGXS := $(shell $(PG_CONFIG) --pgxs)
include $(PGXS)
