-- params: usn, deleted_at, user_id, id
UPDATE processing_notes
SET
  usn = ?,
  updated_at = ?,
  is_deleted = 1
WHERE user_id = ?
  AND id = ?;
