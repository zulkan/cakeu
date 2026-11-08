-- LOSSY: transfer legs are deleted here because the pre-transfer schema cannot
-- represent them. Their balances are not restored anywhere, so rolling this
-- migration back changes account balances by the amount of every transfer.
DELETE FROM transactions WHERE kind = 'transfer';

DROP INDEX IF EXISTS idx_transactions_user_transfer_group;

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_kind_check;
ALTER TABLE transactions
    ADD CONSTRAINT transactions_kind_check
    CHECK (kind IN ('normal', 'opening', 'adjustment'));

ALTER TABLE transactions DROP COLUMN IF EXISTS transfer_group_id;
