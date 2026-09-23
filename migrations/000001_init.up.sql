CREATE TABLE users (
    id UUID PRIMARY KEY,
    name TEXT,
    role TEXT CHECK (role IN ('ADMIN', 'OPERATOR')),
    created_at TIMESTAMPTZ
);

CREATE TABLE machines (
    id UUID PRIMARY KEY,
    model TEXT,
    status TEXT,
    current_operator_id UUID REFERENCES users
);

CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    category TEXT,
    machine_id UUID REFERENCES machines,
    operator_id UUID REFERENCES users,
    status TEXT,
    target_volume NUMERIC,
    estimated_minutes INT
);

CREATE TABLE anomalies (
    id UUID PRIMARY KEY,
    operator_id UUID REFERENCES users,
    machine_id UUID REFERENCES machines,
    type TEXT,
    severity TEXT,
    timestamp TIMESTAMPTZ
);

CREATE TABLE training_modules (
    id UUID PRIMARY KEY,
    title TEXT,
    duration_minutes INT,
    trigger_tag TEXT
);

CREATE TABLE operator_training (
    id UUID PRIMARY KEY,
    operator_id UUID REFERENCES users,
    module_id UUID REFERENCES training_modules,
    status TEXT,
    score NUMERIC
);
