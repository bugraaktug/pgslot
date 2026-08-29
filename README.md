# pgslot

Replication slot WAL health, history, and diagnostics for PostgreSQL.

Read-only: no `Drop Slot`, no `advance_slot`, no mutation of any kind. pgslot
tells you what's wrong; you decide what to do about it.

## Why pgslot

A replication slot with no consumer keeping up doesn't fail loudly -- it
just quietly retains WAL until the disk fills up, and by the time
`pg_replication_slots` looks alarming, it's often already an incident.
pgslot snapshots slot state on a timer, computes real consumption rates
from consecutive snapshots (not an assumed interval), and turns that into a
`status`/`reason` any dashboard or on-call runbook can read directly --
without granting anything broader than `SELECT` on a handful of views.

* **Real elapsed-time rates** -- WAL growth and consumer progress are
  computed from the actual time between snapshots via `lag()`, not a
  fixed polling-interval assumption.
* **Storage/API split** -- the raw history table is `REVOKE ALL FROM
  PUBLIC`; every consumer reads through views, so storage can change
  across versions without breaking dashboards or the CLI.
* **Adapter contract** -- anything downstream of a slot (Walkrie, a
  Debezium connector, a custom sink) can report its own pipeline state via
  `pgslot.report_metric()`, joined against Postgres-side slot health in
  `pgslot.slot_pipeline`.
* **No restart required to install** -- the extension itself is plain
  SQL/PLpgSQL. Whether you *run* collection via the background worker
  (restart required) or an external scheduler (no restart) is a deploy-time
  choice, not an install-time one.

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

**Recommended: the background worker** (`src/`) -- runs its own timer inside
Postgres and calls `pgslot.collect()`/`pgslot.prune()` for you. Requires
`shared_preload_libraries = 'pgslot'` and a restart:

```bash
cd src && make && sudo make install
```

```
# postgresql.conf
shared_preload_libraries = 'pgslot'
pgslot.database = 'yourdb'
pgslot.role     = 'pgslot_cron'   # a role in pgslot_collector, e.g. from Install above
```

See `src/README.md` for the full GUC list and config.

**Don't want to restart Postgres?** Call `pgslot.collect()` yourself on
whatever external schedule you've already got -- this needs no
`shared_preload_libraries` and no restart, ever:

```bash
# cron / systemd timer
*/1 * * * *  psql -d yourdb -U pgslot_cron -c "SELECT pgslot.collect();"

# and periodically:
0 3 * * *    psql -d yourdb -U pgslot_cron -c "SELECT pgslot.prune(168);"
```

or a k8s `CronJob` running the equivalent `psql` command. Both paths write
to the same `slot_history` table and are safe to switch between.

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

## CLI & TUI

Two ways to watch live slot and pipeline state beyond one-off SQL queries,
both Go binaries over the same read-only views -- no separate install path,
no broader grants than `pgslot_monitor` already gives you.

- **CLI** (`cli/`, see `cli/README.md`) -- `pgslot health` / `slots` /
  `watch` / `history <slot>` / `pipeline` / `pipeline-history <slot>` /
  `publications`. `pgslot watch` auto-refreshes in place for a live view
  from a script or a plain terminal.
- **TUI** (`ui/`, see `ui/README.md`) -- an interactive terminal dashboard
  (pgAdmin-style: a connections tree alongside live-polling Overview/
  Publications/detail tabs) for watching multiple slots' health and
  adapter pipeline state at once, with per-slot history trends a click
  away, instead of hand-typing SQL per slot.

## License

pgslot is licensed under the [Apache License, Version 2.0](./LICENSE).
The CLI depends on [`lib/pq`](https://github.com/lib/pq) (BSD 3-Clause).
The TUI depends on [`rivo/tview`](https://github.com/rivo/tview) (MIT) and
[`gdamore/tcell`](https://github.com/gdamore/tcell) (Apache 2.0).

Developed with AI assistance via [Claude Code](https://claude.com/claude-code).

---
