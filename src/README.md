# pgslot background worker

**Autonomous, in-database collection for pgslot.**

Runs `pgslot.collect()` (and `pgslot.prune()`) on its own timer, inside
Postgres, via `RegisterBackgroundWorker()` -- so a deployment that doesn't
want to run an external scheduler at all doesn't have to. Built and tested
live: compiled clean under PG15 and run against a real database with active
logical replication slots, ticking `collect()` on schedule and correctly
computing consumption rates from the resulting snapshots.

This is the **recommended** way to run collection (see the root
[README](../README.md#collect)) -- calling `pgslot.collect()` yourself on an
external cron/systemd/k8s schedule remains a fully supported fallback for
anyone who doesn't want to restart Postgres to enable
`shared_preload_libraries`.

## Why a background worker

An external scheduler is one more moving part to deploy, monitor, and keep
in sync with the database it's collecting from -- a missed cron run, a
misconfigured k8s CronJob, or a systemd timer nobody remembers exists are
all silent failure modes. A worker registered via
`shared_preload_libraries` lives and dies with the postmaster: it restarts
automatically on crash (`bgw_restart_time`), needs no separate process
supervision, and there's nothing external to misconfigure once
`pgslot.database`/`pgslot.role` are set.

* **`WaitLatch`-driven timer loop**, not a busy poll -- sleeps for
  `pgslot.collect_interval_ms` between ticks, woken early and harmlessly by
  spurious latch sets (normal Postgres bgworker behavior).
* **Independent prune cadence** -- `pgslot.prune()` runs on its own slower
  timer (`pgslot.prune_interval_ms`), not once per `collect()` tick.
* **No default role** -- `pgslot.role` must be set explicitly; the worker
  `FATAL`s at startup rather than silently connecting as bootstrap
  superuser.
* **One SPI call per tick, in its own transaction** -- errors (extension
  not yet created in this database, role misconfigured) are logged as a
  `WARNING` and retried next interval rather than crashing the worker.

## Build

Kept in its own `Makefile`, deliberately separate from the top-level one --
the extension's own `make install` stays pure SQL, no C toolchain required
just to install pgslot itself.

```bash
cd src
make && sudo make install
```

## Configure

`pgslot.role` needs a **real login role that's a member of
`pgslot_collector`** -- the `pgslot_collector` role itself, defined in
`../scripts/roles.sql`, is `NOLOGIN` and can't be connected as directly.
`roles.sql` doesn't create a login role for you; wire one yourself, e.g.:

```sql
CREATE ROLE pgslot_worker_svc LOGIN IN ROLE pgslot_collector;
-- no password needed: BackgroundWorkerInitializeConnection() connects
-- in-process, bypassing pg_hba/libpq authentication entirely
```

(`roles.sql`'s own commented example at the bottom does the same thing
under the name `pgslot_cron` -- pick either name, or your own; what matters
is `LOGIN IN ROLE pgslot_collector`.)

All GUCs live in `pgslot_config.h`/`.c`:

| GUC | Default | Notes |
|---|---|---|
| `pgslot.database` | unset | Worker stays unregistered until this is set. |
| `pgslot.role` | unset | No default -- `FATAL`s rather than using bootstrap superuser. Must be `LOGIN IN ROLE pgslot_collector`. |
| `pgslot.collect_interval_ms` | `60000` | |
| `pgslot.prune_interval_ms` | `3600000` | `0` disables worker-driven pruning. |
| `pgslot.prune_keep_hours` | `168` | Passed as `pgslot.prune(keep_hours)`'s argument. |

```
# postgresql.conf -- restart required
shared_preload_libraries = 'pgslot'
pgslot.database = 'yourdb'
pgslot.role     = 'pgslot_worker_svc'
```

## Status

Working, one open limitation: a single worker instance connects to exactly
one database (`BackgroundWorkerInitializeConnection` takes a single
dbname), so a cluster running pgslot across several databases needs more
than one registered worker -- not solved yet. The SQL API
(`pgslot.collect()`, the views, grants) is identical either way; the worker
just calls the same function internally instead of an external scheduler
calling it.

## License

Part of pgslot, licensed under the [Apache License, Version 2.0](../LICENSE).
