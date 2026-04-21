CREATE TABLE scheduled_task_runs (
    scheduled_run_id TEXT PRIMARY KEY,
    scheduled_task_id TEXT NOT NULL,
    mode TEXT NOT NULL,
    scheduled_for TEXT NOT NULL,
    attempt INTEGER NOT NULL,
    status TEXT NOT NULL,
    codex_thread_id TEXT NULL,
    workdir_path TEXT NULL,
    temp_worktree_path TEXT NULL,
    started_at TEXT NULL,
    finished_at TEXT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    response_text TEXT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (scheduled_task_id, scheduled_for, attempt),
    FOREIGN KEY (scheduled_task_id) REFERENCES scheduled_tasks(scheduled_task_id) ON DELETE CASCADE
);

CREATE INDEX scheduled_task_runs_task_due_idx
    ON scheduled_task_runs (scheduled_task_id, scheduled_for DESC, attempt DESC);

CREATE TABLE scheduled_task_deliveries (
    scheduled_delivery_id TEXT PRIMARY KEY,
    scheduled_run_id TEXT NOT NULL,
    discord_channel_id TEXT NOT NULL,
    discord_message_id TEXT NULL,
    status TEXT NOT NULL,
    delivered_at TEXT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (scheduled_run_id) REFERENCES scheduled_task_runs(scheduled_run_id) ON DELETE CASCADE
);

CREATE INDEX scheduled_task_deliveries_run_idx
    ON scheduled_task_deliveries (scheduled_run_id, created_at ASC);
