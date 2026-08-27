/* pgslot_config.c -- GUC registration for the phase-2 background worker */

#include "postgres.h"
#include "utils/guc.h"

#include <limits.h>

#include "pgslot_config.h"

int   pgslot_collect_interval_ms = 60000;
int   pgslot_prune_interval_ms = 3600000;  /* hourly */
int   pgslot_prune_keep_hours = 168;       /* 7 days, matches prune()'s own SQL default */
char *pgslot_worker_database = NULL;
char *pgslot_worker_role = NULL;

void pgslot_config_init(void)
{
    DefineCustomIntVariable("pgslot.collect_interval_ms",
                             "Interval between automatic pgslot.collect() calls.",
                             NULL, &pgslot_collect_interval_ms,
                             60000, 1000, INT_MAX,
                             PGC_SIGHUP, 0, NULL, NULL, NULL);

    DefineCustomIntVariable("pgslot.prune_interval_ms",
                             "Interval between automatic pgslot.prune() calls; 0 disables.",
                             NULL, &pgslot_prune_interval_ms,
                             3600000, 0, INT_MAX,
                             PGC_SIGHUP, 0, NULL, NULL, NULL);

    DefineCustomIntVariable("pgslot.prune_keep_hours",
                             "keep_hours argument passed to pgslot.prune() by the worker.",
                             NULL, &pgslot_prune_keep_hours,
                             168, 1, INT_MAX,
                             PGC_SIGHUP, 0, NULL, NULL, NULL);

    DefineCustomStringVariable("pgslot.database",
                                "Database the pgslot background worker connects to.",
                                NULL, &pgslot_worker_database,
                                NULL, PGC_POSTMASTER, 0, NULL, NULL, NULL);

    DefineCustomStringVariable("pgslot.role",
                                "Role the pgslot background worker connects as (needs pgslot_collector).",
                                NULL, &pgslot_worker_role,
                                NULL, PGC_POSTMASTER, 0, NULL, NULL, NULL);
}
