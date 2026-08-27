/* pgslot_worker.c -- 
 * phase-2 background worker, calls collect()/prune() via SPI on its own timer */

#include "postgres.h"

#include "miscadmin.h"
#include "postmaster/bgworker.h"
#include "storage/ipc.h"
#include "storage/latch.h"
#include "storage/proc.h"
#include "utils/guc.h"
#include "utils/snapmgr.h"
#include "utils/timestamp.h"
#include "utils/wait_event.h"
#include "executor/spi.h"

#include "pgslot_config.h"

PG_MODULE_MAGIC;

void _PG_init(void);
PGDLLEXPORT void pgslot_main(Datum main_arg) pg_attribute_noreturn();

static volatile sig_atomic_t got_sigterm = false;

static void pgslot_sigterm(SIGNAL_ARGS)
{
    int save_errno = errno;
    got_sigterm = true;
    SetLatch(MyLatch);
    errno = save_errno;
}

/* Runs one SQL statement via SPI in its own transaction; logs and swallows errors so a bad tick doesn't kill the worker. */
static void pgslot_spi_exec(const char *sql)
{
    int ret;

    SetCurrentStatementStartTimestamp();
    StartTransactionCommand();
    SPI_connect();
    PushActiveSnapshot(GetTransactionSnapshot());

    ret = SPI_execute(sql, false, 0);
    if (ret != SPI_OK_SELECT)
        ereport(WARNING, (errmsg("pgslot worker: \"%s\" failed (SPI code %d)", sql, ret)));

    SPI_finish();
    PopActiveSnapshot();
    CommitTransactionCommand();
}

/* One worker instance services one database -- see pgslot_config.h on pgslot.database. */
void pgslot_main(Datum main_arg)
{
    TimestampTz next_prune = 0;

    pqsignal(SIGTERM, pgslot_sigterm);
    BackgroundWorkerUnblockSignals();
    BackgroundWorkerInitializeConnection(pgslot_worker_database, pgslot_worker_role, 0);

    ereport(LOG, (errmsg("pgslot worker started on \"%s\" as \"%s\"",
                          pgslot_worker_database, pgslot_worker_role)));

    if (pgslot_prune_interval_ms > 0)
        next_prune = TimestampTzPlusMilliseconds(GetCurrentTimestamp(), pgslot_prune_interval_ms);

    while (!got_sigterm)
    {
        int rc;

        pgslot_spi_exec("SELECT pgslot.collect()");

        if (pgslot_prune_interval_ms > 0 && GetCurrentTimestamp() >= next_prune)
        {
            char sql[64];
            snprintf(sql, sizeof(sql), "SELECT pgslot.prune(%d)", pgslot_prune_keep_hours);
            pgslot_spi_exec(sql);
            next_prune = TimestampTzPlusMilliseconds(GetCurrentTimestamp(), pgslot_prune_interval_ms);
        }

        rc = WaitLatch(MyLatch, WL_LATCH_SET | WL_TIMEOUT | WL_EXIT_ON_PM_DEATH,
                        pgslot_collect_interval_ms, PG_WAIT_EXTENSION);
        ResetLatch(MyLatch);

        if (rc & WL_POSTMASTER_DEATH)
            proc_exit(1);
    }

    proc_exit(0);
}

void _PG_init(void)
{
    BackgroundWorker worker;

    if (!process_shared_preload_libraries_in_progress)
        return;

    pgslot_config_init();

    if (pgslot_worker_database == NULL || pgslot_worker_database[0] == '\0')
        return;   /* stay a pure SQL/PLpgSQL extension until pgslot.database is set */

    if (pgslot_worker_role == NULL || pgslot_worker_role[0] == '\0')
        ereport(FATAL, (errmsg("pgslot.database is set but pgslot.role is not")));

    memset(&worker, 0, sizeof(worker));
    worker.bgw_flags = BGWORKER_SHMEM_ACCESS | BGWORKER_BACKEND_DATABASE_CONNECTION;
    worker.bgw_start_time = BgWorkerStart_RecoveryFinished;
    worker.bgw_restart_time = 10;
    snprintf(worker.bgw_library_name, BGW_MAXLEN, "pgslot");
    snprintf(worker.bgw_function_name, BGW_MAXLEN, "pgslot_main");
    snprintf(worker.bgw_name, BGW_MAXLEN, "pgslot collector");
    snprintf(worker.bgw_type, BGW_MAXLEN, "pgslot collector");
    worker.bgw_main_arg = (Datum) 0;
    worker.bgw_notify_pid = 0;

    RegisterBackgroundWorker(&worker);
}
