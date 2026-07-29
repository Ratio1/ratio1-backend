# Didit KYC/KYB migration runbook

This runbook prepares the one-time Ratio1 cutover from Sumsub onboarding to Didit. It does not authorize a production deployment or database write. Production execution requires an approved change window, a fresh backup, reviewed dry-run evidence, named operators, and explicit approval for the exact counts.

## Safety invariants

- New applicants use exactly one provider. Sumsub session creation is disabled when Didit is activated.
- Existing `approved` and `finalRejected` records are preserved and marked as Sumsub.
- Every other KYC state is reset to `accCreated`, marked as Didit, and starts a new Didit session.
- `user_infos` are preserved for the approved/final-rejected cohort and deleted only for the reset cohort, so stale invoicing identities cannot survive a new verification.
- The internal KYC UUID is Didit `vendor_data`.
- Webhook arrival order is never state truth. Store and deduplicate `event_id`, then reconcile the authoritative Didit decision.
- Grandfathered Sumsub monitoring events persist only the normalized transition
  fields required for restart recovery; raw webhook bodies are not stored.
- Country restrictions remain the approved Sumsub-equivalent lists.
- Minimum age remains jurisdiction-specific, including country and state overrides. Do not introduce a global age of 18.

## Preconditions

Before scheduling the cutover:

1. Merge and deploy the schema and backend support with Didit session creation inactive.
2. Confirm the `kycs.verification_provider` column exists. The cutover CLI deliberately does not run `AutoMigrate`.
3. Publish and lock the production KYC and KYB workflows and record their exact versions.
4. Confirm all KYC/KYB features and questionnaire versions. KYC billing fields remain questionnaire-backed; the KYB questionnaire contains Source-of-Funds fields only.
5. Confirm document, residence and company-country restrictions.
6. In both sandbox and production KYC workflows, click Didit's **Apply age of majority**, review every country and state/region override, and configure below-minimum results to **Decline**. The current sandbox workflow was observed with a global age of 18 and is an explicit activation blocker until corrected.
7. In the native KYB Registry Fields configuration, confirm company name, country and region are collected; set `legal_address` and `tax_number` to Required and `vat_number` to Required (Didit enforces it conditionally for EU companies). Do not duplicate these fields in the Source-of-Funds questionnaire.
8. Complete Sandbox acceptance and production Try Webhook tests.
9. Confirm a stable public HTTPS webhook URL and fixed dApp callback URL.
10. Confirm provider billing balance, ongoing-monitoring settings and alerts.
11. Announce a verification maintenance window that prevents KYC creation and mutation during the transaction.

## Production secrets and provider configuration

Keep all values in the production secret store. Never put values in Git, a PR, Notion, chat, shell history or frontend `VITE_` variables.

Required runtime configuration:

- Active verification provider: `didit`
- `DIDIT_API_URL`
- Didit production environment/application identifier
- `DIDIT_API_KEY`
- production KYC workflow ID and expected version
- production KYB workflow ID and expected version
- production KYC questionnaire billing-field IDs and expected version
- production KYB Source-of-Funds questionnaire ID, expected version and required answer IDs
- `DIDIT_WEBHOOK_SECRET`
- stable Didit webhook URL
- fixed Ratio1 callback URL

Use a least-privilege runtime API key. Keep management, workflow-edit and bulk-delete permissions on a separate operator credential. Keep Sumsub credentials only for the restricted grandfathered-monitoring path.

Create a V3 Didit webhook destination and subscribe to:

- `status.updated`
- `data.updated`
- `user.status.updated`
- `user.data.updated`
- `business.status.updated`
- `business.data.updated`

Store the destination secret when it is created; it is shown once. Didit currently documents `18.203.201.92` as its delivery IP. Allow it through the WAF if required, but never replace timestamp and HMAC verification with an IP check. Prefer `X-Signature-V2`; raw HMAC is an authenticated fallback. A Simple signature may only trigger an authoritative re-fetch and must not make a decision payload trusted.

## Backup and read-only dry run

Take a fresh, restorable database backup and record its identifier outside this repository. Confirm restoration has been rehearsed.

The CLI reads either the existing ignored `DATABASE_LINK` value in `host:port:database:user:password` form or the five standard `DATABASE_*` variables. Remote connections require `DATABASE_SSLMODE=require`, `verify-ca`, or `verify-full`. Do not print or source credentials.

Build and run the dry-run from the repository root:

```bash
go build -o /tmp/ratio1-verification-cutover ./cmd/verification-cutover
/tmp/ratio1-verification-cutover
```

Dry-run is the default. It starts a read-only transaction, prints aggregates only, and exits without updating data. It refuses:

- unknown KYC statuses;
- duplicate UUID or email groups;
- null, blank or whitespace-altered identities;
- any pre-existing Didit provider row, because that indicates Didit traffic has already started;
- any approved KYC row without a matching `user_infos` identity;
- supplied expected counts that do not exactly match.

Optionally compare a reviewed expectation during dry-run:

```bash
/tmp/ratio1-verification-cutover \
  --expected-total <total> \
  --expected-preserved <approved-plus-final-rejected> \
  --expected-reset <all-other-statuses> \
  --expected-reset-user-infos <stale-reset-cohort-user-infos>
```

A prior read-only snapshot contained 215 KYC rows: 84 preserved and 131 reset. This is a historical example only, not an apply default or an acceptable substitute for a fresh cutover-window reading.

Review and archive:

- total, approved, final-rejected, preserved and reset counts;
- every status aggregate;
- current provider aggregates;
- duplicate and invalid-identity counts;
- total `user_infos`, reset-cohort `user_infos`, and approved rows missing `user_infos`;
- number of reset rows containing legacy applicant/country/VIES data.

## Apply transaction

Do not apply while the API can create or mutate KYC records.

The apply command requires all four reviewed counts:

```bash
/tmp/ratio1-verification-cutover \
  --apply \
  --expected-total <total> \
  --expected-preserved <approved-plus-final-rejected> \
  --expected-reset <all-other-statuses> \
  --expected-reset-user-infos <stale-reset-cohort-user-infos>
```

Apply behavior:

1. Opens a serializable transaction.
2. Locks all current KYC rows.
3. Re-runs every preflight check and exact-count comparison.
4. Marks preserved `approved`/`finalRejected` records as `sumsub`.
5. Resets all other records to `accCreated`.
6. Clears reset rows' Sumsub applicant ID, applicant type, country and VIES flag.
7. Marks reset rows as `didit`.
8. Deletes only `user_infos` matched to reset-cohort KYC records and verifies the exact reviewed deletion count.
9. Re-runs aggregates and verifies preserved identities remain while no reset-cohort `user_infos` remain.
10. Commits only if every invariant passes.

Archive the before/after output and transaction result. A non-zero exit means no successful commit unless the output explicitly contains `result=applied_and_committed`; investigate before any retry.

## Maintenance-window cutover

1. Enter verification maintenance mode.
2. Confirm no KYC writes or new provider sessions are occurring.
3. Take the final backup.
4. Run and approve the final dry-run.
5. Apply with the exact final counts.
6. Activate the server-side Didit provider switch.
7. Disable Sumsub session creation.
8. Keep Didit webhook processing active.
9. Keep only grandfathered Sumsub ongoing monitoring active.
10. Deploy/activate the provider-neutral dApp.
11. Complete the smoke tests below.
12. Exit maintenance and begin heightened monitoring.

Opening or merging the migration PRs must not itself activate production traffic.

## Acceptance and smoke tests

At minimum verify:

- Existing approved individual and business accounts remain approved, can read their profile, and retain license/purchase access.
- Existing final-rejected accounts remain blocked and cannot start a session.
- A reset individual can choose KYC, receives one Didit session, completes the hosted flow, and reaches the expected internal state.
- A reset company can choose KYB, supplies full invoicing identity, completes the hosted flow, and reaches the expected internal state.
- Duplicate clicks/reloads resume one active session.
- A terminal declined, expired or abandoned Didit session remains blocked until
  authoritative reconciliation marks both the session and local KYC retryable;
  the next request then creates one replacement session. `finalRejected` never
  becomes retryable.
- Restricted document, residence and company countries fail as designed.
- The Didit **Apply age of majority** policy is active in sandbox and production, below-minimum action is Decline, and representative 18/19/20/21 country/state boundaries behave as configured.
- Retryable document, liveness, proof-of-address and KYB-document failures permit retry.
- Confirmed AML/sanctions, blocked entities and jurisdictional minimum-age failures revoke access.
- `FLAGGED` maps to `onHold`; `BLOCKED` maps to `finalRejected`; `ACTIVE` never grants approval by itself.
- V2/raw/Simple webhook paths, exact replay, payload mismatch, out-of-order events and provider timeouts behave safely.
- Approval creates complete `UserInfo`; tax/VAT and VIES behavior match invoicing requirements.
- KYB invoicing data comes from approved native registry company data; the questionnaire contains only Source-of-Funds answers.
- Approved/rejected email notifications use a durable transition key and are
  normally deduplicated, but delivery is at-least-once: a process crash after
  the mail provider accepts a message and before the database marks it sent can
  produce a duplicate.
- Mobile/desktop callback, refresh, back navigation and delayed webhook UX work.

## Monitoring and alerts

Monitor without logging webhook bodies or names, emails, documents, addresses or tax IDs:

- session creation successes, latency and errors by environment/type;
- signature failures, stale timestamps and oversized webhook bodies;
- received, duplicate, mismatch, processed, failed and dead-letter events;
- webhook queue depth and oldest-event age;
- reconciliation latency and Didit `401`, `403`, `429` and `5xx`;
- policy/workflow mismatch, incomplete evidence and unknown warning projections;
- abnormal approval, rejection and on-hold rates;
- notification failures, retries, deduplication and sent-but-unacknowledged
  crash windows; manually reconcile ambiguous deliveries with the mail
  provider before replaying;
- no-webhook-with-active-sessions;
- provider balance/credit;
- grandfathered Sumsub monitoring events and approaching retirement dates.

Keep runbooks for event replay, stuck queue recovery, manual reconciliation, secret rotation, policy drift, provider outage and compliance review of `onHold`.

## Rollback

Before any Didit session is created:

1. Re-enter maintenance.
2. Disable Didit.
3. Restore the reviewed database backup or rollback the verified migration according to the approved change plan.
4. Re-enable Sumsub onboarding only after confirming the dApp/backend contract and legacy state.

After any Didit traffic exists:

1. Disable new verification session creation.
2. Continue accepting and processing existing Didit webhooks.
3. Keep affected users in their current safe internal states.
4. Fix forward or perform reviewed manual remediation.

Never send pending Didit applicants back into Sumsub automatically. Doing so creates overlapping provider identities and ambiguous state.

## Privacy, legal and retention gates

Before production approval:

- Execute/review the Didit DPA, technical and organizational measures, subprocessors, processing region and incident terms.
- Confirm Ratio1's legal basis and controller responsibilities.
- Update the Ratio1 privacy notice and verification disclosures, including links to Didit's end-user privacy notice and terms.
- Document data categories, AML/KYB processing, automated checks, transfers, appeal/human-review paths and data-subject contacts.
- Configure an explicit Didit retention period; do not leave the unlimited default unintentionally.
- Decide whether to process and purge Didit sessions after extracting the minimum compliance/invoicing evidence.
- Document erasure, export and provider-session deletion procedures.
- Define Ratio1 retention for provider/session IDs, event hashes, projection/audit metadata and `UserInfo`.
- Define rejected/minor-applicant retention.
- Confirm the compliance basis, duration and cost of ongoing monitoring.

## Grandfathered Sumsub monitoring retirement

Sumsub remains only for ongoing monitoring of preserved approved users until their approved expiry/re-verification policy is satisfied.

- Do not create new Sumsub sessions.
- Do not let legacy Sumsub events approve, reset or make a final rejection retryable.
- Allow validated monitoring events only to suspend or revoke access.
- Track the remaining grandfathered population and its retirement date.
- Re-verify in Didit when policy requires.
- When the last grandfathered record is retired, disable the Sumsub destination, rotate/remove credentials, delete the legacy monitoring code in a reviewed follow-up, and confirm no Sumsub traffic remains.
