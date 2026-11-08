-- Budgets are an amount in exactly one currency: (user_id, category_id, month, currency).
-- Existing rows never recorded a currency, so the backfill is a documented best guess.
ALTER TABLE budgets ADD COLUMN currency VARCHAR(3);

-- Backfill precedence:
--   1. most frequent account currency among the user's normal expense
--      transactions in the same category and month (tie-break: currency ASC);
--   2. otherwise the currency of the user's earliest account (created_at, id);
--   3. otherwise 'USD'.
UPDATE budgets b
SET currency = COALESCE(
    (
        SELECT ranked.currency
        FROM (
            SELECT a.currency, COUNT(*) AS occurrences
            FROM transactions t
            JOIN accounts a ON a.id = t.account_id
            WHERE t.user_id = b.user_id
              AND t.category_id = b.category_id
              AND t.type = 'expense'
              AND t.kind = 'normal'
              AND t.date >= b.month
              AND t.date < (b.month + INTERVAL '1 month')
            GROUP BY a.currency
            ORDER BY occurrences DESC, a.currency ASC
            LIMIT 1
        ) AS ranked
    ),
    (
        SELECT a.currency
        FROM accounts a
        WHERE a.user_id = b.user_id
        ORDER BY a.created_at, a.id
        LIMIT 1
    ),
    'USD'
);

ALTER TABLE budgets ALTER COLUMN currency SET NOT NULL;

ALTER TABLE budgets DROP CONSTRAINT budgets_user_id_category_id_month_key;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_user_id_category_id_month_currency_key
    UNIQUE (user_id, category_id, month, currency);

CREATE INDEX IF NOT EXISTS idx_budgets_user_month_currency
    ON budgets(user_id, month, currency);
