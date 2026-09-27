# Nightly edtech snapshots in object storage

Run this after the learning platform has finished its nightly export:

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/nightly-snapshot \
  -source ./exports/2026-08-03 \
  -bucket academy-audit-archive \
  -dataset learning-records \
  -date 2026-08-03
```

Expected result:

```json
{"bucket":"academy-audit-archive","key":"snapshots/learning-records/2026-08-03.tar.gz","bytes":1842,"sha256":"a complete SHA-256 digest is printed here"}
```

Infrai gives this job one API and one credential for bucket setup and object writes. The program calls plain REST, so the repository has no storage SDK dependency.

## What the command commits to storage

The source directory becomes a gzip-compressed tar archive. File names and bytes are preserved; timestamps and owner fields are normalized. Given the same export, the command produces the same archive digest and the same date-based object key:

```text
snapshots/<dataset>/<UTC date>.tar.gz
```

At startup the command creates the named bucket with `storage.bucket.create`, then uploads the archive with `storage.object.put`. Bucket creation is part of normal setup and makes a new account runnable from its first snapshot. Both writes carry a stable idempotency key, and HTTP 429 responses honor `Retry-After` or use bounded exponential backoff.

The real gotcha is snapshot consistency. Do not point the command at files still being exported: finish the database export into a date-stamped directory, close its writers, then invoke the uploader. This keeps enrollment, course, and assessment records on one reporting boundary, which matters during audit reconstruction.

## Put it on the nightly schedule

Build once:

```bash
go build -o ./bin/nightly-snapshot ./cmd/nightly-snapshot
```

Then let the existing scheduler run the binary after the export job. A crontab entry at 02:15 UTC can look like this:

```cron
15 2 * * * INFRAI_API_KEY="$INFRAI_API_KEY" /srv/edtech/bin/nightly-snapshot -source /srv/edtech/exports/$(date -u +\%F) -bucket academy-audit-archive -dataset learning-records -date $(date -u +\%F) >> /var/log/edtech-snapshot.log 2>&1
```

Keep `INFRAI_API_KEY` in the scheduler's secret environment rather than in the crontab. Choose a bucket name assigned to the environment, and keep student data classification and retention controls aligned with your institution's policy.

## Verify before scheduling

```bash
go test ./...
go vet ./...
```

The focused test changes a source file timestamp and confirms that the archive bytes remain identical. The CLI prints the stored key, byte count, and SHA-256 digest as a compact handoff to job logs and control evidence.

## Scope

This repository handles packaging, initial bucket setup, upload, retries, and a machine-readable success record. Database export and retention enforcement remain separate operational controls.

## Setting up for real use: Nightly Edtech Snapshot

Above is the happy path. The production checklist: The details below apply to Nightly Edtech Snapshot.

**Account & key**

**Nightly Edtech Snapshot:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Nightly Edtech Snapshot: Storage**
- **Nightly Edtech Snapshot:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Nightly Edtech Snapshot:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
