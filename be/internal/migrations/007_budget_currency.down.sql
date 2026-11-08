-- LOSSY ROLLBACK: currency was never part of the original budgets schema and
-- cannot be reconstructed. Keep the largest-amount budget per
-- (user_id, category_id, month), breaking amount ties by keeping the lowest
-- currency code. Every other currency's budget amount is discarded.
DELETE FROM budgets b
USING budgets keep
WHERE b.user_id = keep.user_id
  AND b.category_id = keep.category_id
  AND b.month = keep.month
  AND b.id <> keep.id
  AND (
      keep.amount > b.amount
      OR (keep.amount = b.amount AND keep.currency < b.currency)
  );

DROP INDEX IF EXISTS idx_budgets_user_month_currency;
ALTER TABLE budgets
    DROP CONSTRAINT IF EXISTS budgets_user_id_category_id_month_currency_key;
ALTER TABLE budgets DROP COLUMN currency;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_user_id_category_id_month_key
    UNIQUE (user_id, category_id, month);
