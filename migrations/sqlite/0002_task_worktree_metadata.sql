ALTER TABLE tasks ADD COLUMN branch_name TEXT NOT NULL DEFAULT '';
ALTER TABLE tasks ADD COLUMN base_ref TEXT NULL;
ALTER TABLE tasks ADD COLUMN worktree_path TEXT NULL;
ALTER TABLE tasks ADD COLUMN worktree_status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE tasks ADD COLUMN worktree_created_at TEXT NULL;
ALTER TABLE tasks ADD COLUMN worktree_pruned_at TEXT NULL;
ALTER TABLE tasks ADD COLUMN last_used_at TEXT NULL;

UPDATE tasks
SET branch_name = 'task/' || task_id
WHERE branch_name = '';
