ALTER TABLE scheduled_tasks RENAME COLUMN report_channel_id TO report_target;

ALTER TABLE scheduled_task_deliveries RENAME COLUMN discord_channel_id TO report_target;
