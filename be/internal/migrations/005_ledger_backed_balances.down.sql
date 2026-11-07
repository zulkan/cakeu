ALTER TABLE accounts ADD COLUMN balance NUMERIC(15,2) NOT NULL DEFAULT 0;

UPDATE accounts a
SET balance = COALESCE((
    SELECT SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END)
    FROM transactions t
    WHERE t.account_id = a.id
), 0);

DELETE FROM transactions WHERE kind <> 'normal';

DROP INDEX IF EXISTS idx_transactions_kind;
DROP INDEX IF EXISTS idx_transactions_account_kind;

ALTER TABLE transactions DROP COLUMN kind;
ALTER TABLE transactions ALTER COLUMN category_id SET NOT NULL;
