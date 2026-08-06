# PostgreSQL to CockroachDB migration runbook

This runbook records the DEV rehearsal for migrating the Ratio1 backend from
PostgreSQL to CockroachDB. Mainnet must not be accessed or changed until a
separate, explicit mainnet migration approval is given.

## Safety rules

- Never print, log, commit, or pass a complete database link on a command line.
- Do not `source` `.env`. Read only the explicitly approved variable.
- Store dumps in a directory with mode `700`; store dump, SQL, and verification
  files with mode `600`.
- Do not start the full backend to create the schema. Full startup also reads a
  private key, initializes external configuration and templates, and starts the
  HTTP server.
- Run initial schema creation only against an empty target. Before enabling
  normal startup migration, verify the exact GORM and PostgreSQL-driver versions
  with repeated `AutoMigrate` runs against the target CockroachDB version.
- The validated application combination is `gorm.io/gorm v1.31.2` with
  `gorm.io/driver/postgres v1.6.0`. Because the application supplies a
  `lib/pq` `*sql.DB`, `postgres.Config.DriverName` must be set to `postgres`.
- For a cutover, stop source writers or otherwise define the exact consistent
  snapshot and change-freeze window before taking the final dump.

The links currently use this repository-specific format:

```text
host:port:database:user:password
```

Parse the value inside the shell process and unset it immediately after use.
Do not echo any parsed field.

## 1. Preflight

1. Confirm the approved source and target variables are present without
   printing their values.
2. Connect to both databases read-only with SSL, a connection timeout, a
   statement timeout, and `ON_ERROR_STOP`.
3. Record database engine versions.
4. Inventory source tables, columns, constraints, indexes, sequences, and row
   counts.
5. Inventory the target and confirm whether it is empty. Do not overwrite an
   unexpected target.
6. Confirm the local PostgreSQL client tools are compatible with the source.

DEV rehearsal variables:

```text
source: DEV_DATABASE_LINK
target: COCKROACH_DEV_DATABASE_LINK
```

Do not substitute mainnet variables until the mainnet run is separately
approved.

## 2. Create the local source snapshot

Create a permission-restricted migration directory and use `pg_dump` directly
against the approved source. The DEV rehearsal used PostgreSQL custom format,
no ownership, no grants, and a serializable deferrable snapshot:

```sh
umask 077
MIGRATION_DIR="$(mktemp -d /tmp/ratio1-postgres-to-cockroach.XXXXXX)"
chmod 700 "$MIGRATION_DIR"

pg_dump \
  --format=custom \
  --compress=9 \
  --no-owner \
  --no-privileges \
  --serializable-deferrable \
  --file="$MIGRATION_DIR/source.dump" \
  "<password-free libpq connection parameters>"

chmod 600 "$MIGRATION_DIR/source.dump"
pg_restore --list "$MIGRATION_DIR/source.dump" >/dev/null
shasum -a 256 "$MIGRATION_DIR/source.dump"
```

Keep the checksum, byte size, source engine version, snapshot time, and row
counts as migration evidence. The dump contains customer data and must not be
committed.

## 3. Reset only the approved CockroachDB target

CockroachDB does not allow dropping its built-in `public` schema in this
environment. Drop only the known application objects, transactionally:

```sql
DROP TABLE IF EXISTS
  public.account_notification_emails,
  public.accounts,
  public.allocations,
  public.brandings,
  public.burn_events,
  public.invoice_clients,
  public.invoice_drafts,
  public.kycs,
  public.preferences,
  public.sellers,
  public.stats,
  public.user_infos
  CASCADE;

DROP SEQUENCE IF EXISTS
  public.allocations_id_seq,
  public.burn_events_id_seq
  CASCADE;
```

After the transaction commits, verify that `public` contains zero application
tables and zero application sequences.

## 4. Create the target schema from GORM models

Use the existing `storage.Connect()` / `storage.TryMigrate()` path through a
temporary one-shot Go program. Configure `config.Config.Database` directly and
do not load the full application configuration.

The temporary runner must use at least two open connections. One connection
deadlocks with GORM v1.23.2 because its existing-table inspection can keep a
catalog result open while issuing a second catalog query.

Use:

```text
MaxOpenConns: 2
MaxIdleConns: 1
SslMode: require
```

Run it with this session option so GORM's explicit `integer` columns retain
PostgreSQL-compatible 32-bit width:

```sh
PGOPTIONS='-c default_int_size=4 -c statement_timeout=120000' \
  ./temporary-automigrate-runner
```

Run the initial `AutoMigrate` once against the clean target. If initial schema
creation fails or is interrupted, reset the target again before retrying rather
than resuming against a partial schema. After loading and verifying data, run
the same migration path at least twice and confirm it emits no DDL before
allowing normal application startup.

The previous GORM v1.23.2/PostgreSQL-driver v1.3.1 combination was unsafe on
repeat startup. CockroachDB exposes the columns in the composite partial unique index
`idx_allocation_tx_log(tx_hash, log_index)` as individually `UNIQUE` through
the catalog query used by those versions. The old PostgreSQL migrator then
attempted to add incorrect single-column unique constraints during a subsequent
`AutoMigrate`. GORM v1.31.2 with PostgreSQL driver v1.6.0 was verified to retain
only the intended partial composite unique index over three consecutive local
runs, two one-shot CockroachDB DEV runs, and a full backend startup.

The modern migrator performs many per-column catalog queries. Against the
remote DEV CockroachDB cluster, a no-op migration took a few minutes and logged
many slow catalog queries. Treat this as expected startup latency, keep a
per-statement timeout, and ensure deployments allow enough startup time.

CockroachDB v23.1 translates GORM's `bigserial` columns to `INT8 DEFAULT
unique_rowid()` in its default `rowid` mode. This is an intentional
Cockroach-native replacement for PostgreSQL sequences and avoids distributed
sequence coordination. See the CockroachDB v23.1
[`SERIAL` documentation](https://www.cockroachlabs.com/docs/v23.1/serial).

## 5. Schema acceptance gate

Do not copy data until all checks pass:

- 12 application tables exist.
- The application tables contain zero rows.
- No hidden `rowid` columns exist.
- All 128 visible source columns have matching target names, data types, and
  nullability. Column order is not required to match because the data load uses
  explicit column lists.
- `invoice_drafts.invoice_number`, `preferences.next_number`, and
  `stats.daily_active_jobs` are `integer`, not `bigint`.
- All 12 primary keys exist.
- All 6 foreign keys exist and are validated.
- The partial unique index `idx_allocation_tx_log(tx_hash, log_index) WHERE
  log_index IS NOT NULL` exists. A single-column unique constraint on
  `allocations.tx_hash` must not exist.
- All expected secondary indexes exist.
- `allocations.id` and `burn_events.id` are `INT8 DEFAULT unique_rowid()`.
- No PostgreSQL sequence objects exist in the target when using CockroachDB's
  default `rowid` serial-normalization mode.

Current-model differences from the existing PostgreSQL schema:

- GORM creates redundant unique indexes for `kycs.uuid` and
  `stats.creation_timestamp` in addition to their primary keys. This follows
  the current `gorm:"primarykey;unique"` model tags and does not change accepted
  values.
- The PostgreSQL source has database defaults `false` on
  `invoice_clients.reverse_charge` and `true` on `invoice_clients.is_ue`.
  Current GORM models do not declare those defaults, so a clean CockroachDB
  AutoMigrate does not recreate them. Normal application inserts explicitly
  write both Go boolean fields, so this does not block copying existing data,
  but exact schema parity should be decided before the mainnet migration.

## 6. Data load

Restore data only, never the PostgreSQL schema. Load referenced tables before
referencing tables:

1. `user_infos`
2. `accounts`
3. `account_notification_emails`
4. `allocations`
5. `brandings`
6. `burn_events`
7. `invoice_clients`
8. `invoice_drafts`
9. `kycs`
10. `preferences`
11. `sellers`
12. `stats`

Do not include the PostgreSQL `SEQUENCE SET` archive entries because the GORM
CockroachDB schema uses `unique_rowid()` and has no sequences.

On CockroachDB v23.1, direct custom-archive `pg_restore` COPY streaming failed
during the DEV rehearsal. Render the selected table-data entries to a
permission-restricted plain SQL file, inspect only its COPY headers, and load it
with `psql --single-transaction --set=ON_ERROR_STOP=1`. The SQL file contains
customer data and must remain mode `600`.

## 7. Data verification

After loading:

1. Compare exact per-table row counts at the source snapshot and target.
2. Export every table from both databases in primary-key order through the
   PostgreSQL wire protocol into mode-`600` local files.
3. Compare files without printing contents.
4. Treat result-order differences separately from value differences.
5. Compare JSON columns semantically because PostgreSQL and CockroachDB may
   serialize JSON object keys in different orders.
6. Recheck constraints, indexes, hidden columns, ID defaults, and total rows.
7. Run a focused application smoke test only after structural and data checks
   pass.

CockroachDB v23.1 did not support a complete PostgreSQL `pg_dump` from the
installed libpq client because its `pg_extension` compatibility view lacks a
catalog column queried by `pg_dump`. Before a schema-changing operation, retain
all of the following protected rollback artifacts instead:

- a fresh PostgreSQL source custom archive;
- a complete ordered CockroachDB row export;
- per-table `SHOW CREATE TABLE` output;
- index and constraint inventories.

Use a native CockroachDB backup as an additional safeguard when the deployment
provides an approved backup destination.

## DEV rehearsal checkpoint

As of 2026-08-06:

- PostgreSQL DEV was dumped locally with a consistent custom-format snapshot.
- CockroachDB DEV was reset.
- A clean, one-pass GORM AutoMigrate completed successfully.
- CockroachDB DEV has 12 application tables, zero application rows, zero hidden
  `rowid` columns, zero sequences, and 128 source-compatible visible columns
  immediately before the data load.
- All 6 foreign keys are validated.
- The live PostgreSQL DEV source had not drifted from the local snapshot before
  loading.
- The sequence-free data load committed successfully in one transaction.
- All 451 rows match PostgreSQL field-by-field across all 12 tables; JSON values
  were compared semantically.
- Post-load verification confirmed 12 primary keys, 6 validated foreign keys,
  all source indexes, zero hidden `rowid` columns, and zero PostgreSQL sequence
  objects.
- A fresh read-only export proved that PostgreSQL DEV was unchanged from the
  migration snapshot and CockroachDB DEV was unchanged from the verified load,
  so no reset or data reload was needed.
- Fresh protected PostgreSQL and CockroachDB logical rollback artifacts were
  created before testing the upgraded migration path.
- GORM was upgraded to v1.31.2 and the PostgreSQL driver to v1.6.0. Both
  `lib/pq`-backed GORM connection sites explicitly set `DriverName: "postgres"`.
- Two one-shot `AutoMigrate` passes against CockroachDB DEV completed with zero
  errors and emitted zero DDL. Table definitions, indexes, constraints, and all
  451 rows were unchanged before and after.
- A full local backend process using CockroachDB DEV reached HTTP readiness,
  served a read-only branding lookup with HTTP 200, emitted zero DDL, and was
  stopped gracefully. The data remained byte-identical afterward.
- Repeated automatic migration is no longer a DEV cutover blocker. Deploying
  the changed backend configuration remains an operational deployment step.
- Mainnet has not been accessed or changed.
