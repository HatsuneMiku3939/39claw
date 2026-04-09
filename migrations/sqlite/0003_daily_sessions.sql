CREATE TABLE daily_sessions (
    local_date TEXT NOT NULL,
    generation INTEGER NOT NULL,
    logical_thread_key TEXT NOT NULL,
    previous_logical_thread_key TEXT NULL,
    activation_reason TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (local_date, generation),
    UNIQUE (logical_thread_key)
);

CREATE UNIQUE INDEX daily_sessions_active_local_date_idx
    ON daily_sessions (local_date)
    WHERE is_active = 1;

UPDATE thread_bindings
SET logical_thread_key = logical_thread_key || '#1'
WHERE mode = 'daily'
  AND instr(logical_thread_key, '#') = 0
  AND length(logical_thread_key) = 10
  AND substr(logical_thread_key, 5, 1) = '-'
  AND substr(logical_thread_key, 8, 1) = '-';

INSERT OR IGNORE INTO daily_sessions (
    local_date,
    generation,
    logical_thread_key,
    previous_logical_thread_key,
    activation_reason,
    is_active,
    created_at,
    updated_at
)
SELECT
    substr(logical_thread_key, 1, 10) AS local_date,
    1 AS generation,
    logical_thread_key,
    NULL,
    'legacy-migration',
    1,
    created_at,
    updated_at
FROM thread_bindings
WHERE mode = 'daily'
  AND instr(logical_thread_key, '#') > 0
  AND substr(logical_thread_key, length(logical_thread_key), 1) = '1'
  AND length(logical_thread_key) > 11;
