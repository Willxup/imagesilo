DROP INDEX sessions_expires_at_idx;

CREATE TABLE sessions_new (
    id TEXT PRIMARY KEY,
    admin_id TEXT NOT NULL REFERENCES admin(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    csrf_hash BLOB NOT NULL CHECK (length(csrf_hash) = 32),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;

CREATE UNIQUE INDEX sessions_token_hash_idx ON sessions(token_hash);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);
