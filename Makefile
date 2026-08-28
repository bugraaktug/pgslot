EXTENSION   = pgslot
DATA        = sql/pgslot--0.3.sql sql/pgslot--0.1--0.2.sql sql/pgslot--0.2--0.3.sql
PGFILEDESC  = "pgslot - replication slot WAL health, history, and diagnostics"

PG_CONFIG ?= pg_config
PGXS := $(shell $(PG_CONFIG) --pgxs)
include $(PGXS)
