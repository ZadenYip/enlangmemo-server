-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 collection.usn 升序拉取当前 Pull 会话范围内的 collection 变更。
SELECT
  id,
  usn,
  sqlite_schema_version,
  created_at,
  updated_at,
  config
FROM collections
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;
