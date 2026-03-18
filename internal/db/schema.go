package db

// EmbeddingDim is the vector dimension of multilingual-e5-small (384-dim).
const EmbeddingDim = 384

const schema = `
CREATE TABLE IF NOT EXISTS processed_files (
    id          INTEGER PRIMARY KEY,
    file_name   TEXT NOT NULL,
    file_hash   TEXT NOT NULL UNIQUE,
    imported_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chunks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    text        TEXT NOT NULL,
    is_leader   INTEGER NOT NULL DEFAULT 0,
    start_time  DATETIME,
    end_time    DATETIME,
    file_hash   TEXT REFERENCES processed_files(file_hash)
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
    chunk_id  INTEGER PRIMARY KEY,
    embedding FLOAT[384]
);

CREATE TABLE IF NOT EXISTS message_hashes (
    msg_hash TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS messages (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    speaker   TEXT NOT NULL,
    content   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_date ON messages(date(timestamp));
`
