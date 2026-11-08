ALTER TABLE transactions ADD COLUMN transfer_group_id UUID;

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_kind_check;
ALTER TABLE transactions
    ADD CONSTRAINT transactions_kind_check
    CHECK (kind IN ('normal', 'opening', 'adjustment', 'transfer'));

CREATE INDEX IF NOT EXISTS idx_transactions_user_transfer_group
    ON transactions(user_id, transfer_group_id)
    WHERE transfer_group_id IS NOT NULL;
