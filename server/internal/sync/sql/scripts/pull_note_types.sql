-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 note_types.usn 升序拉取当前 Pull 会话范围内的 note_type 变更。
SELECT
  id,
  usn,
  CASE WHEN is_deleted = 1 THEN NULL ELSE name END AS name,
  CASE WHEN is_deleted = 1 THEN NULL ELSE preset_template_id END AS preset_template_id,
  updated_at,
  CASE WHEN is_deleted = 1 THEN NULL ELSE note_template END AS note_template,
  is_deleted
FROM note_types
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;
