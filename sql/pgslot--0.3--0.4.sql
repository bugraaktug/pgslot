-- pgslot--0.3--0.4.sql
-- Adds available_publications: a read-only diagnostic view over
-- pg_publication, so a cluster's publication config can be inspected
-- alongside slot data. Informational only -- pg_replication_slots does not
-- record which publication a slot consumes (that mapping lives only in
-- subscriber/adapter config), so this view cannot be joined to slot_health.
--
-- No table/column changes, no grants here -- roles are cluster-wide and
-- deliberately kept out of extension scripts (see scripts/roles.sql); after
-- this upgrade, re-run scripts/roles.sql to add SELECT on the new view to
-- pgslot_monitor (it's idempotent, safe to re-run any time).

\echo Use "ALTER EXTENSION pgslot UPDATE TO '0.4'" to apply this upgrade. \quit

CREATE VIEW available_publications AS
SELECT
    p.pubname,
    p.pubowner::regrole::text AS owner,
    p.puballtables,
    p.pubinsert,
    p.pubupdate,
    p.pubdelete,
    p.pubtruncate,
    p.pubviaroot
FROM pg_publication p
ORDER BY p.pubname;

COMMENT ON VIEW available_publications IS
    'Read-only wrapper over pg_publication -- informational only, cannot '
    'be joined to slot_health since pg_replication_slots does not record '
    'which publication a slot consumes (that mapping lives only in '
    'subscriber/adapter config).';
