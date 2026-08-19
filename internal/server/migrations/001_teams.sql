CREATE TABLE IF NOT EXISTS teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    developer_name TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, device_id)
);

CREATE TABLE IF NOT EXISTS synced_intents (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    repo_fingerprint TEXT NOT NULL,
    file_path TEXT NOT NULL,
    what TEXT NOT NULL,
    why TEXT NOT NULL,
    git_ref TEXT,
    developer_name TEXT NOT NULL,
    vector_clock JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_synced_intents_team_repo ON synced_intents(team_id, repo_fingerprint);
