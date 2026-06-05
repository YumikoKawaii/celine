CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE clients (
    sub          TEXT PRIMARY KEY,
    email        TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    memory_md    TEXT,
    preferences  JSONB        NOT NULL DEFAULT '{}',
    persona_note TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE conversations (
    id         TEXT        PRIMARY KEY,
    owner_sub  TEXT        NOT NULL REFERENCES clients(sub) ON DELETE CASCADE,
    project_id BIGINT,
    title      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON conversations (owner_sub, created_at DESC);

CREATE TABLE messages (
    id              TEXT        PRIMARY KEY,
    conversation_id TEXT        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT        NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON messages (conversation_id, created_at ASC);

CREATE TABLE memory_index (
    id         BIGSERIAL   PRIMARY KEY,
    owner_sub  TEXT        NOT NULL REFERENCES clients(sub)  ON DELETE CASCADE,
    message_id TEXT        NOT NULL REFERENCES messages(id)  ON DELETE CASCADE,
    role       TEXT        NOT NULL CHECK (role IN ('user', 'assistant')),
    content    TEXT        NOT NULL,
    embedding  vector(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (message_id)
);

CREATE INDEX ON memory_index (owner_sub);
