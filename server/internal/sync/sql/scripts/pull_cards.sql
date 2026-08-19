-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 cards.usn 升序拉取当前 Pull 会话范围内的 card 变更。
SELECT
  id,
  usn,
  CASE WHEN is_deleted = 1 THEN NULL ELSE note_id END AS note_id,
  CASE WHEN is_deleted = 1 THEN NULL ELSE deck_id END AS deck_id,
  updated_at,
  CASE WHEN is_deleted = 1 THEN NULL ELSE difficulty END AS difficulty,
  CASE WHEN is_deleted = 1 THEN NULL ELSE stability END AS stability,
  CASE WHEN is_deleted = 1 THEN NULL ELSE scheduled_days END AS scheduled_days,
  CASE WHEN is_deleted = 1 THEN NULL ELSE due END AS due,
  CASE WHEN is_deleted = 1 THEN NULL ELSE last_review END AS last_review,
  CASE WHEN is_deleted = 1 THEN NULL ELSE lapses END AS lapses,
  CASE WHEN is_deleted = 1 THEN NULL ELSE learning_steps END AS learning_steps,
  CASE WHEN is_deleted = 1 THEN NULL ELSE repetitions END AS repetitions,
  CASE WHEN is_deleted = 1 THEN NULL ELSE state END AS state,
  CASE WHEN is_deleted = 1 THEN NULL ELSE queue END AS queue,
  is_deleted
FROM cards
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;
