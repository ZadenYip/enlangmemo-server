-- params: user_id, entity_id, entity_type, op, usn, updated_at
INSERT INTO sync_units (
  user_id,
  entity_id,
  entity_type,
  op,
  usn,
  updated_at
) VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  entity_type = VALUES(entity_type),
  op = VALUES(op),
  usn = VALUES(usn),
  updated_at = VALUES(updated_at);
