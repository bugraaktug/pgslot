# pgslot UI

A read-only terminal UI over pgslot's read-only views -- same contract as
the extension and CLI: never mutates a slot. Built with
[tview](https://github.com/rivo/tview) (pure Go, no cgo, no Node/npm),
reusing `cli/pg`'s existing `Connect`/`Fetch*` functions directly rather
than duplicating any query code.

## Screenshot

The Overview tab, connected to a real multi-slot cluster (`pgslot pipeline`'s
data, live-polled every 2s):

```
╔═════════ Connections ════════╗┌───────────────────────────────────────────────── pgslot ─────────────────────────────────────────────────┐
║Connections                   ║│ 8 slots (0 active) - 832.1 MB retained  |  OK:0 WARN:2 CRIT:6  |  updated 20:06:05                       │
║└──qdb  ●                     ║│SLOT                            STATUS   ACTIVE RETAINED ADAPTER (latest)              EVENTS/SEC QUEUE   │
║                              ║│cdc_backfill_slot               CRITICAL no     111.3 MB walkrie                       0          0     ● │
║                              ║│cdc_slot                        WARNING  no     32.1 MB  walkrie                       0          0     ● │
║                              ║│cdc_slot_qdrant                 CRITICAL no     110.7 MB n/a                           n/a        n/a   ● │
║                              ║│cv_cdc_slot                     CRITICAL no     226.5 MB n/a                           n/a        n/a   ● │
║                              ║│docker_smoke_slot               CRITICAL no     113.1 MB n/a                           n/a        n/a   ● │
║                              ║│pgcdc_slot                      WARNING  no     32.1 MB  walkrie                       0          0     ● │
║                              ║│product_cdc_slot                CRITICAL no     100.4 MB n/a                           n/a        n/a   ● │
║                              ║│walkrie_it_lsn_slot             CRITICAL no     105.9 MB n/a                           n/a        n/a   ● │
║                              ║│                                                                                                          │
║                              ║│                                                                                                          │
║                              ║│                                                                                                          │
╚══════════════════════════════╝└──────────────────────────────────────────────────────────────────────────────────────────────────────────┘
add  delete  enter connect/open  tab switch pane  1 overview  2 publications  esc close tab  quit
```

## Build

```bash
cd ui && make build      # ./pgslot-ui
make install               # installs to $PREFIX/bin (default /usr/local/bin)
```

Kept out of the top-level `Makefile`, same reasoning as `src/Makefile` and
`cli/Makefile`: the extension's own `make install` stays pure SQL.

## Connect

Saved connection profiles live in a local JSON file at
`$XDG_CONFIG_HOME/pgslot-ui/connections.json` (typically
`~/.config/pgslot-ui/connections.json`), single-machine, not synced
anywhere:

```json
[
  { "name": "qdb", "dsn": "host=localhost port=5432 dbname=qdb user=pgslot_monitor password=..." }
]
```

Connect as a role granted `pgslot_monitor` (see `../scripts/roles.sql`) --
the UI only ever needs `SELECT` on the views, same as the CLI.

## Keys

```
a          add a connection (opens a form)
d          remove the selected connection
enter      connect (on a tree node) / open detail (on a table row)
tab        switch focus between the tree and the content pane
1  /  2    jump to the Overview / Publications tab
esc        close the current detail tab (or cancel a form/dialog)
q          quit
```

## Layout

pgAdmin-style: a persistent connections tree on the left, tabbed pages on
the right.

- **Overview** -- `slot_pipeline`, refreshed every 2s (matches the CLI's
  `watch -interval` default), htop-style: a live header line (OK/WARN/CRIT
  counts + a "last updated" clock) proves it's actively polling, not a
  static snapshot. STATUS and ACTIVE are separate columns on purpose: a
  slot can be CRITICAL while still active, or CRITICAL because it's
  inactive and retaining WAL -- the two are independent facts. The
  rightmost column is a colored status dot, kept deliberately faint/simple
  (a full-row background tint was tried first but dominated the table when
  most rows were critical -- a common real case). ADAPTER is labeled
  "(latest)": it's whichever adapter last called `report_metric()` for
  that slot (Walkrie today, but the contract is generic -- see
  `../adapters/README.md`), and `slot_pipeline`'s join only ever surfaces
  the single most recent report per slot.
- **Publications** -- `available_publications`, fetched once per connect,
  not polled. Standalone: `pg_replication_slots` doesn't record which
  publication a slot consumes, so this can't be joined to slot health.
- Selecting a slot row opens a closable **detail** tab (reused if already
  open for that slot), fetched once per open, not polled: `slot_history_rates`
  (WAL growth vs. consumer rate) and `slot_pipeline_history` (adapter
  `events_per_sec`/`queue_depth`) -- independent time axes, not merged.
  Each series gets a compact block-character sparkline (the terminal
  equivalent of a line chart) plus the underlying rows as a scrollable
  table.
