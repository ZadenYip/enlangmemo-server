-- params:
--   user_id, id,
--   note_type_id, usn, created_at, updated_at,
--   sense_id, fields_json
INSERT INTO notes (
  user_id,
  id,
  note_type_id,
  usn,
  created_at,
  updated_at,
  sense_id,
  fields,
  is_deleted
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
ON DUPLICATE KEY UPDATE
  note_type_id = VALUES(note_type_id),
  usn = VALUES(usn),
  created_at = VALUES(created_at),
  updated_at = VALUES(updated_at),
  sense_id = VALUES(sense_id),
  fields = VALUES(fields),
  is_deleted = 0;
