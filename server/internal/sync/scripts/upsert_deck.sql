-- params:
--   user_id, id, usn,
--   name, updated_at,
--   new_cards_per_day, new_learned_today, learned_today, reviewed_today,
--   config_json
INSERT INTO decks (
  user_id,
  id,
  usn,
  name,
  updated_at,
  new_cards_per_day,
  new_learned_today,
  learned_today,
  reviewed_today,
  config,
  is_deleted
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
ON DUPLICATE KEY UPDATE
  usn = VALUES(usn),
  name = VALUES(name),
  updated_at = VALUES(updated_at),
  new_cards_per_day = VALUES(new_cards_per_day),
  new_learned_today = VALUES(new_learned_today),
  learned_today = VALUES(learned_today),
  reviewed_today = VALUES(reviewed_today),
  config = VALUES(config),
  is_deleted = 0;
