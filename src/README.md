# src/ (phase 2 -- background worker)

v0.1 ships no C code. `pgslot.collect()` is a plain SQL/PLpgSQL function
called externally on a timer (systemd, k8s CronJob, manual psql, the pgslot
CLI's own loop) -- no `shared_preload_libraries`, no restart to install.

Phase 2 adds an optional background worker (`pgslot_worker.so`) that runs its
own `WaitLatch` loop and calls the same collection logic autonomously, for
deployments that don't want to run an external scheduler at all. **Built and
tested live** (compiled clean under PG15, ran against a real database with
active logical replication slots, ticking `collect()` on schedule):

- `pgslot_worker.c` -- `_PG_init()` registers the bgworker via `RegisterBackgroundWorker()`;
  `pgslot_main()` loops calling `pgslot.collect()`, and `pgslot.prune()` on its own
  slower cadence, via SPI
- `pgslot_config.h`/`.c` -- the GUCs: `pgslot.collect_interval_ms`,
  `pgslot.prune_interval_ms` (0 disables worker-driven pruning),
  `pgslot.prune_keep_hours`, `pgslot.database`, `pgslot.role`. The worker
  stays unregistered until `pgslot.database` is set; `pgslot.role` has no
  default on purpose (see `pgslot_config.h`) -- FATALs rather than
  connecting as bootstrap superuser

Built with its **own `src/Makefile`** (`MODULE_big`/`OBJS`), deliberately
kept separate from the top-level `Makefile` -- so `make install` at the repo
root stays pure SQL, no Go/C toolchain required just to install pgslot
itself. To build and try the worker:

```bash
cd src && make && sudo make install     # installs pgslot.so onto the PGXS lib path
```

Then in `postgresql.conf` (restart required):

```
shared_preload_libraries = 'pgslot'
pgslot.database = 'yourdb'
pgslot.role     = 'some_role_in_pgslot_collector'
```

Still open: one worker instance connects to exactly one database
(`BackgroundWorkerInitializeConnection` takes a single dbname), so a
cluster running pgslot in several databases needs more than one registered
worker -- not solved yet.

The SQL API (`pgslot.collect()`, the views, grants) does not change when this
lands -- the bgworker just calls the same `collect()` function internally
instead of a cron job calling it externally. Existing installs stay on the
external-scheduler path unless they opt into preloading the library.
