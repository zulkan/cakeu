ALTER TABLE transactions
    ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT 'normal'
    CHECK (kind IN ('normal', 'opening', 'adjustment'));

ALTER TABLE transactions ALTER COLUMN category_id DROP NOT NULL;

INSERT INTO transactions
    (user_id, account_id, category_id, type, amount, description, date, kind)
SELECT
    a.user_id,
    a.id,
    NULL,
    CASE WHEN a.balance - COALESCE(n.net, 0) >= 0 THEN 'income' ELSE 'expense' END,
    ABS(a.balance - COALESCE(n.net, 0)),
    'Opening balance',
    LEAST(a.created_at::date, COALESCE(n.first_date, a.created_at::date)),
    'opening'
FROM accounts a
LEFT JOIN (
    SELECT account_id,
           SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END) AS net,
           MIN(date) AS first_date
    FROM transactions
    GROUP BY account_id
) n ON n.account_id = a.id
WHERE a.balance - COALESCE(n.net, 0) <> 0
  AND NOT EXISTS (
      SELECT 1 FROM transactions t
      WHERE t.account_id = a.id AND t.kind = 'opening'
  );

ALTER TABLE accounts DROP COLUMN balance;

CREATE INDEX idx_transactions_kind ON transactions(kind);
CREATE INDEX idx_transactions_account_kind ON transactions(account_id, kind);
