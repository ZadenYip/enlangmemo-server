-- params: user_id, cursor_usn_inclusive, upper_usn_exclusive, limit
-- 按 review_logs.usn 升序拉取当前 Pull 会话范围内的 review_log 变更。
-- review_log 不支持删除，因此没有 is_deleted 字段。
SELECT
  id,
  usn,
  card_id,
  review_time,
  scheduled_days,
  rating,
  difficulty,
  stability,
  learning_steps,
  state,
  duration
FROM review_logs
WHERE user_id = ?
  AND usn >= ?
  AND usn < ?
ORDER BY usn ASC
LIMIT ?;

