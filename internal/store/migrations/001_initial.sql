CREATE TABLE IF NOT EXISTS intents (
    id           TEXT    PRIMARY KEY,
    project_id   TEXT    NOT NULL,
    file_path    TEXT    NOT NULL,
    branch       TEXT    NOT NULL DEFAULT 'unknown',
    commit_hash  TEXT,
    developer    TEXT    NOT NULL,
    timestamp    INTEGER NOT NULL,
    type         TEXT    NOT NULL
                         CHECK(type IN
                         ('change','new_file','delete','refactor')),
    what         TEXT    NOT NULL,
    why          TEXT    NOT NULL,
    impact       TEXT    DEFAULT '',
    context      TEXT    NOT NULL DEFAULT 'normal'
                         CHECK(context IN ('normal','low','inferred'))
);

CREATE TABLE IF NOT EXISTS file_registry (
    file_path    TEXT    NOT NULL,
    project_id   TEXT    NOT NULL,
    purpose      TEXT    NOT NULL DEFAULT '',
    owns         TEXT    NOT NULL DEFAULT '',
    boundary     TEXT    NOT NULL DEFAULT '',
    created_by   TEXT    NOT NULL,
    created_at   INTEGER NOT NULL,
    PRIMARY KEY (file_path, project_id)
);

CREATE TABLE IF NOT EXISTS file_saves (
    file_path    TEXT    NOT NULL,
    project_id   TEXT    NOT NULL,
    saved_at     INTEGER NOT NULL,
    has_note     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version      INTEGER PRIMARY KEY,
    applied_at   INTEGER NOT NULL,
    description  TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_intents_project
    ON intents(project_id);
CREATE INDEX IF NOT EXISTS idx_intents_file
    ON intents(project_id, file_path);
CREATE INDEX IF NOT EXISTS idx_intents_timestamp
    ON intents(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_intents_developer
    ON intents(developer);
CREATE INDEX IF NOT EXISTS idx_saves_file
    ON file_saves(project_id, file_path);
