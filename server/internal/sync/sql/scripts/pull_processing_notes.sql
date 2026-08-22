-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 processing_notes.usn 升序拉取当前 Pull 会话范围内的 processing_note 变更。
SELECT
  id,
  usn,
  CASE WHEN is_deleted = 1 THEN NULL ELSE note_type_id END AS note_type_id,
  CASE WHEN is_deleted = 1 THEN NULL ELSE created_at END AS created_at,
  updated_at,
  CASE WHEN is_deleted = 1 THEN NULL ELSE sense_id END AS sense_id,
  CASE WHEN is_deleted = 1 THEN NULL ELSE fields END AS fields,
  is_deleted
FROM processing_notes
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;
