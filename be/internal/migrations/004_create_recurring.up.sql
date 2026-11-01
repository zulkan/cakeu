CREATE TABLE IF NOT EXISTS recurring_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    type VARCHAR(10) NOT NULL CHECK (type IN ('income', 'expense')),
    amount NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    description VARCHAR(500) NOT NULL DEFAULT '',
    frequency VARCHAR(10) NOT NULL CHECK (frequency IN ('daily', 'weekly', 'monthly')),
    interval INTEGER NOT NULL DEFAULT 1 CHECK (interval BETWEEN 1 AND 12),
    next_run DATE NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recurring_rules_due ON recurring_rules(active, next_run);
CREATE INDEX IF NOT EXISTS idx_recurring_rules_user ON recurring_rules(user_id);

CREATE TABLE IF NOT EXISTS recurring_runs (
    rule_id UUID NOT NULL REFERENCES recurring_rules(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (rule_id, period_start)
);
