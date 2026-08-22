-- params: sync_cursor_usn, user_id
UPDATE collections
SET sync_cursor_usn = ?
WHERE user_id = ?;
