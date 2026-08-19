-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 decks.usn 升序拉取当前 Pull 会话范围内的 deck 变更。
SELECT
  id,
  usn,
  CASE WHEN is_deleted = 1 THEN NULL ELSE name END AS name,
  updated_at,
  CASE WHEN is_deleted = 1 THEN NULL ELSE new_cards_per_day END AS new_cards_per_day,
  CASE WHEN is_deleted = 1 THEN NULL ELSE new_learned_today END AS new_learned_today,
  CASE WHEN is_deleted = 1 THEN NULL ELSE learned_today END AS learned_today,
  CASE WHEN is_deleted = 1 THEN NULL ELSE reviewed_today END AS reviewed_today,
  CASE WHEN is_deleted = 1 THEN NULL ELSE config END AS config,
  is_deleted
FROM decks
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;
