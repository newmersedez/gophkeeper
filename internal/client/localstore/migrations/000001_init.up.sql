CREATE TABLE meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE items (
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted INTEGER NOT NULL DEFAULT 0,
    dirty INTEGER NOT NULL DEFAULT 0,
    payload BLOB NOT NULL
);
