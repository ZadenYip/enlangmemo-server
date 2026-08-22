-- params:
--   user_id, id,
--   card_id, usn,
--   review_time, scheduled_days,
--   rating, difficulty, stability,
--   learning_steps, state, duration
INSERT INTO review_logs (
  user_id,
  id,
  card_id,
  usn,
  review_time,
  scheduled_days,
  rating,
  difficulty,
  stability,
  learning_steps,
  state,
  duration
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  card_id = VALUES(card_id),
  usn = VALUES(usn),
  review_time = VALUES(review_time),
  scheduled_days = VALUES(scheduled_days),
  rating = VALUES(rating),
  difficulty = VALUES(difficulty),
  stability = VALUES(stability),
  learning_steps = VALUES(learning_steps),
  state = VALUES(state),
  duration = VALUES(duration);
