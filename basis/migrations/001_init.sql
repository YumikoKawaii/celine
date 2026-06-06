CREATE
EXTENSION IF NOT EXISTS vector;

CREATE TABLE prosopons
(
    id           BIGSERIAL PRIMARY KEY,
    sub          TEXT        NOT NULL UNIQUE,
    email        TEXT        NOT NULL,
    display_name TEXT        NOT NULL DEFAULT '',
    avatar_url   TEXT,
    preferences  JSONB       NOT NULL DEFAULT '{}',
    persona      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Celine's own identity record; sub is synthetic (not an OIDC token).
INSERT INTO prosopons (id, sub, email, display_name)
VALUES (1, 'celine', 'celine@internal', 'Celine');

CREATE TABLE conversations
(
    id          BIGSERIAL PRIMARY KEY,
    prosopon_id BIGINT      NOT NULL REFERENCES prosopons (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON conversations (prosopon_id, created_at DESC);

CREATE TABLE messages
(
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT      NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    prosopon_id     BIGINT      NOT NULL,
    content         TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON messages (conversation_id, created_at ASC);

CREATE TABLE memories
(
    id         BIGSERIAL PRIMARY KEY,
    message_id BIGINT      NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    embedding  vector(384) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (message_id)
);

CREATE INDEX ON memories (message_id);
