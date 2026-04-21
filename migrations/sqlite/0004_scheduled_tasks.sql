CREATE TABLE scheduled_tasks (
    scheduled_task_id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    schedule_kind TEXT NOT NULL,
    schedule_expr TEXT NOT NULL,
    prompt TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    report_channel_id TEXT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    disabled_at TEXT NULL
);
