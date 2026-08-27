# pgslot

Replication slot WAL health, history, and diagnostics for PostgreSQL.

Read-only: no `Drop Slot`, no `advance_slot`, no mutation of any kind. pgslot
tells you what's wrong; you decide what to do about it.

## Layout

```
pgslot/
├── pgslot.control          extension metadata
├── Makefile                 PGXS build (pure SQL in v0.1, no C compile step)
├── sql/
│   └── pgslot--0.1.sql      tables, collect()/prune()/report_metric(), views
├── scripts/
│   └── roles.sql             pgslot_monitor / pgslot_collector / pgslot_adapter
├── adapters/
│   └── README.md              reporting contract (Walkrie is the worked example)
├── src/                       phase-2 background worker -- built and tested live,
│   ├── pgslot_worker.c          not part of the top-level `make install`
│   ├── pgslot_config.{h,c}
│   └── Makefile                its own PGXS build (see src/README.md)
├── cli/                       Go CLI -- health/slots/watch/history
│   ├── main.go, cmd_*.go, internal/
│   └── Makefile
└── test/
    └── sql/basic.sql            smoke test
```

## Install

```bash
make install                                    # copies control/sql files onto PGXS path
psql -d yourdb -c "CREATE EXTENSION pgslot;"
psql -d yourdb -f scripts/roles.sql             # creates the NOLOGIN group roles only

# scripts/roles.sql does NOT create any login role -- wire one to each group
# role you actually need (adjust names/passwords for your deployment):
psql -d yourdb -c "CREATE ROLE pgslot_cron    LOGIN PASSWORD '...' IN ROLE pgslot_collector;"
psql -d yourdb -c "CREATE ROLE grafana_reader LOGIN PASSWORD '...' IN ROLE pgslot_monitor;"
```

A database superuser bypasses all of this (Postgres superusers skip GRANT/
REVOKE checks entirely) -- fine for local testing, not recommended for a
real deployment since it's far more privilege than pgslot's views need.

## Collect

No background worker in v0.1 -- wire `pgslot.collect()` into whatever
scheduler you already have, as the `pgslot_cron` role created above:

```bash
# cron / systemd timer
*/1 * * * *  psql -d yourdb -U pgslot_cron -c "SELECT pgslot.collect();"

# and periodically:
0 3 * * *    psql -d yourdb -U pgslot_cron -c "SELECT pgslot.prune(168);"
```

or a k8s `CronJob` running the equivalent `psql` command. See `src/README.md`
for the phase-2 autonomous bgworker path, already built and tested against a
real database with active replication slots.

## Read

```sql
SELECT * FROM pgslot.slot_health;
-- slot_name       | status   | reason
-- walkrie_cdc     | ok       | nominal
-- old_debezium    | critical | inactive slot retaining 381000.0 MB of WAL

SELECT * FROM pgslot.wal_summary;

SELECT * FROM pgslot.slot_pipeline WHERE slot_name = 'walkrie_cdc';
```

## Adapters

See `adapters/README.md`. Any external consumer of a slot (Walkrie, a
Debezium connector, a custom sink) can call `pgslot.report_metric()` to
surface its own pipeline state (`processed_lsn`, `events_per_sec`,
`queue_depth`, ...) joined against Postgres-side slot health in
`pgslot.slot_pipeline`.

## CLI

See `cli/README.md`. `pgslot health` / `pgslot slots` / `pgslot watch` /
`pgslot history <slot>` -- a Go binary over the same read-only views.

---

Developed with AI assistance via [Claude Code](https://claude.com/claude-code).
