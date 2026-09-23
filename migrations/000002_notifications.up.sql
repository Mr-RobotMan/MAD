CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID NOT NULL REFERENCES users(id),
    machine_id UUID NOT NULL REFERENCES machines(id),
    type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('WARNING', 'CRITICAL')),
    timestamp TIMESTAMPTZ NOT NULL
);

CREATE INDEX alerts_operator_severity_idx ON alerts (operator_id, severity);

CREATE TABLE escalations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id UUID NOT NULL UNIQUE REFERENCES alerts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
