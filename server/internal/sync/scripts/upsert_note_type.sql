-- params:
--   user_id, id, usn,
--   name, preset_template_id, updated_at, note_template_json
INSERT INTO note_types (
  user_id,
  id,
  usn,
  name,
  preset_template_id,
  updated_at,
  note_template,
  is_deleted
) VALUES (?, ?, ?, ?, ?, ?, ?, 0)
ON DUPLICATE KEY UPDATE
  usn = VALUES(usn),
  name = VALUES(name),
  preset_template_id = VALUES(preset_template_id),
  updated_at = VALUES(updated_at),
  note_template = VALUES(note_template),
  is_deleted = 0;
