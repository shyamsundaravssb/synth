-- Phase 1 schema additions.
-- Adds embedding storage, daemon state KV store, and full-text search.
-- All statements are idempotent (IF NOT EXISTS) — safe to apply multiple times.

-- ── intent_embeddings ──────────────────────────────────────────────────────
-- Stores the serialised embedding vector for each intent note.
-- The embedding column holds a little-endian binary blob of 384 float32 values.
CREATE TABLE IF NOT EXISTS intent_embeddings (
    intent_id    TEXT    PRIMARY KEY,
    project_id   TEXT    NOT NULL,
    embedding    BLOB    NOT NULL,
    model        TEXT    NOT NULL DEFAULT 'all-MiniLM-L6-v2',
    created_at   INTEGER NOT NULL,
    FOREIGN KEY (intent_id) REFERENCES intents(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_embeddings_project
    ON intent_embeddings(project_id);

-- ── daemon_state ───────────────────────────────────────────────────────────
-- Key-value store for daemon subsystem state that persists across restarts.
CREATE TABLE IF NOT EXISTS daemon_state (
    key          TEXT    PRIMARY KEY,
    project_id   TEXT    NOT NULL,
    value        TEXT    NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_daemon_state_project
    ON daemon_state(project_id);

-- ── intents_fts ────────────────────────────────────────────────────────────
-- Standalone FTS5 full-text search index over intent notes.
-- Kept in sync with the intents table via triggers below.
-- intent_id and project_id are UNINDEXED (stored but not tokenised for search).
-- file_path, what, why, and impact are fully indexed for keyword search.
CREATE VIRTUAL TABLE IF NOT EXISTS intents_fts
    USING fts5(
        intent_id  UNINDEXED,
        project_id UNINDEXED,
        file_path,
        what,
        why,
        impact
    );

-- ── FTS5 sync triggers ─────────────────────────────────────────────────────
-- Keep intents_fts in sync with the intents table automatically.
-- Note: standalone FTS5 tables do not support the 'delete' command; we use
-- DELETE ... WHERE rowid = (subquery) instead.

CREATE TRIGGER IF NOT EXISTS intents_fts_insert
    AFTER INSERT ON intents BEGIN
        INSERT INTO intents_fts(
            intent_id, project_id,
            file_path, what, why, impact)
        VALUES (
            new.id, new.project_id,
            new.file_path, new.what, new.why,
            new.impact);
    END;

CREATE TRIGGER IF NOT EXISTS intents_fts_update
    AFTER UPDATE ON intents BEGIN
        DELETE FROM intents_fts
            WHERE rowid = (
                SELECT rowid FROM intents_fts
                WHERE intent_id = old.id);
        INSERT INTO intents_fts(
            intent_id, project_id,
            file_path, what, why, impact)
        VALUES (
            new.id, new.project_id,
            new.file_path, new.what, new.why,
            new.impact);
    END;

CREATE TRIGGER IF NOT EXISTS intents_fts_delete
    BEFORE DELETE ON intents BEGIN
        DELETE FROM intents_fts
            WHERE rowid = (
                SELECT rowid FROM intents_fts
                WHERE intent_id = old.id);
    END;
