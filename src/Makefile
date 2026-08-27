# Builds/installs pgslot_worker.so in isolation, without touching the
# top-level Makefile's v0.1 "no C compile step" promise. Usage:
#   cd src && make && make install
#   # then in postgresql.conf: shared_preload_libraries = 'pgslot'
#   #                          pgslot.database = 'yourdb'
#   #                          pgslot.role = 'pgslot_collector'   -- restart required

MODULE_big = pgslot
OBJS = pgslot_worker.o pgslot_config.o
PGFILEDESC = "pgslot background worker (phase 2, not shipped in v0.1)"

PG_CONFIG ?= pg_config
PGXS := $(shell $(PG_CONFIG) --pgxs)
include $(PGXS)
