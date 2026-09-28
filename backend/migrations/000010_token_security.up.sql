CREATE TABLE app.token_security_audits (
    coin_id TEXT NOT NULL,
    chain TEXT NOT NULL,
    contract_address TEXT NOT NULL,
    supported BOOLEAN NOT NULL,
    issues JSONB NOT NULL,
    audited_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (coin_id, chain)
);
