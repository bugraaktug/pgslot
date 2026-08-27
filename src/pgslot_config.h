/* pgslot_config.h -- GUCs shared by the phase-2 background worker */
#ifndef PGSLOT_CONFIG_H
#define PGSLOT_CONFIG_H

extern int pgslot_collect_interval_ms;

extern int pgslot_prune_interval_ms;   /* 0 disables worker-driven pruning */
extern int pgslot_prune_keep_hours;

extern char *pgslot_worker_database;   /* unset -> worker stays unregistered */
extern char *pgslot_worker_role;       /* unset -> _PG_init() FATALs, no superuser default */

extern void pgslot_config_init(void);  /* registers the above via DefineCustomXVariable(), call once from _PG_init() */

#endif /* PGSLOT_CONFIG_H */
