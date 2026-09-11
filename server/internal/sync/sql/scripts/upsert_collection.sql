-- params:
--   user_id, id, usn,
--   sqlite_schema_version, daily_reset_time, time_zone, created_at, updated_at, config_json, is_deleted
INSERT INTO collections (
  user_id,
  id,
  usn,
  sqlite_schema_version,
  daily_reset_time,
  time_zone,
  created_at,
  updated_at,
  config,
  is_deleted
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  id = VALUES(id),
  usn = VALUES(usn),
  sqlite_schema_version = VALUES(sqlite_schema_version),
  daily_reset_time = VALUES(daily_reset_time),
  time_zone = VALUES(time_zone),
  created_at = VALUES(created_at),
  updated_at = VALUES(updated_at),
  config = VALUES(config),
  is_deleted = VALUES(is_deleted);
