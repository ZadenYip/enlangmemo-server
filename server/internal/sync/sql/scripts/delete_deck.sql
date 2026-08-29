-- params: usn, deleted_at, user_id, id
UPDATE decks
SET
  usn = ?,
  updated_at = ?,
  is_deleted = 1
WHERE user_id = ?
  AND id = ?
  AND is_deleted = 0;
