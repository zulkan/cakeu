# Transaction Attachments — Design

Date: 2026-11-08
Status: Approved (user selected "attachments only" on 2026-11-08)

## Scope

Receipts and supporting files attached to a transaction: upload, list, download, delete.
Splits and FX are explicitly out of scope.

## Decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| 1 | Storage | Local disk for bytes, Postgres for metadata | No external service or new dependency. Swappable later behind a `Store` interface. |
| 2 | Directory | `ATTACHMENTS_DIR`, default `./data/attachments`, created at startup; startup fails if it cannot be created | A silent failure would produce 500s on the first upload. |
| 3 | On-disk name | Generated UUID plus a sanitized extension; the original filename lives only in the DB | No user input ever reaches the path, so traversal is structurally impossible. |
| 4 | Limits | 5 MB per file; allow `image/jpeg`, `image/png`, `image/webp`, `image/heic`, `application/pdf` | Receipts only. Anything else is 415. |
| 5 | Ownership | Every metadata query is scoped by `user_id`; another user's attachment is a 404, never a 403 | No existence leak. |
| 6 | Download | Streamed with the stored content type and `Content-Disposition: inline`; filename stripped of CR/LF/quotes | A bearer token cannot be sent by a plain link, so the SPA fetches a blob. |
| 7 | Deletion | Delete the DB row, then remove the file; a failed file removal is logged, not fatal | Metadata is the source of truth; an orphan file is harmless. |
| 8 | Cascade cleanup | Transaction and account deletion collect storage keys before deleting, then remove files after the DB succeeds | The FK cascade removes rows; without this the files would leak. |
| 9 | Listing | No `attachment_count` on the transaction list | Avoids changing the hot list query. The UI fetches on demand. |
| 10 | Attachment targets | Any transaction owned by the caller; the UI only offers it on `kind='normal'` rows | System entries have no receipt to attach. |

## Data model — migration 009

```sql
CREATE TABLE transaction_attachments (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    filename       TEXT NOT NULL,
    content_type   TEXT NOT NULL,
    size_bytes     BIGINT NOT NULL CHECK (size_bytes > 0),
    storage_key    TEXT NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_transaction_attachments_user_transaction
    ON transaction_attachments(user_id, transaction_id);
```

Down: drop the index and the table (the files are removed by the application, not the migration).

## API

| Method | Path | Result |
|---|---|---|
| POST | `/api/transactions/{id}/attachments` | 201 metadata after a multipart `file` upload |
| GET | `/api/transactions/{id}/attachments` | 200 array of metadata |
| GET | `/api/attachments/{id}` | 200 binary stream |
| DELETE | `/api/attachments/{id}` | 204 |

Metadata JSON: `{ "id", "transaction_id", "filename", "content_type", "size_bytes", "created_at" }`.

Error mapping: 400 missing file, 404 unknown transaction/attachment, 413 over 5 MB, 415 unsupported
content type, 401 unauthenticated.

## Backend structure

- `internal/attachment` (or `internal/storage`): `Store` interface with `Save(key string, r io.Reader) (int64, error)`,
  `Open(key string) (io.ReadCloser, error)`, `Remove(key string) error`; a local-disk implementation with `MkdirAll`.
- `service/attachment.go`: `AttachmentService` with `List`, `Upload`, `Open`, `Delete`; validates the transaction
  belongs to the caller, the declared size, and the content type.
- `db/attachment.go`: `ListByTransaction`, `GetByID`, `Create`, `Delete`, plus
  `StorageKeysByTransaction` and `StorageKeysByAccount` for cascade cleanup.
- `http/attachment.go`: handlers and streaming. `NewHandler` gains an `AttachmentService` parameter; update
  `cmd/server/main.go` and every test constructor call.
- `config.Config` gains `AttachmentsDir string`; `cmd/server/main.go` constructs the store and fails fast if the
  directory cannot be created.
- `be/Dockerfile`: create and own `/app/data`; `docker-compose.yml`: mount a named volume at `/app/data`; `.gitignore`: add `be/data/`.

## Frontend

- Types: `Attachment` and an upload result; `api.listAttachments`, `api.uploadAttachment`,
  `api.deleteAttachment`, `api.downloadAttachment` (returns a `Blob` using the authenticated fetch path).
- `AttachmentsModal.tsx`: list (filename, human-readable size, date), upload (accept the allowed types, 5 MB
  client-side check), download, delete with `ConfirmDialog`; loading/empty/error states; toasts.
- A paperclip action on `kind='normal'` transaction rows (desktop table and mobile list) opens the modal.
- No changes to transaction list queries.

## Testing

- Backend, DB-free: fake store + fake repo — type rejection, size rejection, ownership 404, upload writes
  metadata and bytes, delete removes both, transaction/account deletion removes collected keys.
- Backend HTTP: 401, 201, 200 list, 204 delete, 404, 413, 415.
- Optional: validate migration 009 against an ephemeral Postgres and exercise the real repo.
- Frontend: upload/delete/download flows, size/type validation, empty state, paperclip visibility.

## Out of scope

Split transactions, OCR/text extraction, image thumbnails, S3/object storage, sharing links, virus scanning.
