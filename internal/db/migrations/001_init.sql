CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    phone TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    bank_account TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE contracts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID REFERENCES users(id),
    freelancer_id UUID REFERENCES users(id),
    title TEXT NOT NULL,
    content_type TEXT NOT NULL,
    duration TEXT NOT NULL,
    format TEXT NOT NULL,
    language TEXT NOT NULL,
    style TEXT[] DEFAULT '{}',
    scenes JSONB NOT NULL DEFAULT '[]',
    reference_links TEXT,
    avoid_notes TEXT,
    amount NUMERIC(10,3) NOT NULL,
    deposit_amount NUMERIC(10,3) NOT NULL,
    revision_count INT NOT NULL DEFAULT 2,
    state TEXT NOT NULL DEFAULT 'PENDING_FREELANCER',
    dispute_reason TEXT,
    admin_decision TEXT,
    fabric_tx_id TEXT,
    brief_hash TEXT NOT NULL,
    deliverable_hash TEXT,
    deadline TIMESTAMPTZ NOT NULL,
    auto_release_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID REFERENCES contracts(id),
    amount NUMERIC(10,3) NOT NULL,
    direction TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'bank_transfer',
    status TEXT NOT NULL DEFAULT 'pending',
    bank_reference TEXT,
    confirmed_by UUID REFERENCES users(id),
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID REFERENCES contracts(id),
    uploader_id UUID REFERENCES users(id),
    original_name TEXT NOT NULL,
    stored_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL
);
