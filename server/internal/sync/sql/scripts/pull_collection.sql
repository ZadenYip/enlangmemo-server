-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 collection.usn 升序拉取当前 Pull 会话范围内的 collection 变更。
SELECT
  id,
  usn,
  CASE WHEN is_deleted = 1 THEN NULL ELSE sqlite_schema_version END AS sqlite_schema_version,
  CASE WHEN is_deleted = 1 THEN NULL ELSE created_at END AS created_at,
  updated_at,
  CASE WHEN is_deleted = 1 THEN NULL ELSE config END AS config,
  is_deleted
FROM collections
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;
