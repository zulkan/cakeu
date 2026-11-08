-- Files on disk are removed by the application, not by this migration.
DROP INDEX IF EXISTS idx_transaction_attachments_user_transaction;

DROP TABLE IF EXISTS transaction_attachments;
