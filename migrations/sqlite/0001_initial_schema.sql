CREATE TABLE IF NOT EXISTS thread_bindings (
    mode TEXT NOT NULL,
    logical_thread_key TEXT NOT NULL,
    codex_thread_id TEXT NOT NULL,
    task_id TEXT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (mode, logical_thread_key)
);

CREATE TABLE IF NOT EXISTS tasks (
    task_id TEXT PRIMARY KEY,
    discord_user_id TEXT NOT NULL,
    task_name TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    closed_at TEXT NULL
);

CREATE TABLE IF NOT EXISTS active_tasks (
    discord_user_id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
