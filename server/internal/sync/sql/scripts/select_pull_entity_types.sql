-- 参数按每个 entity_type 分支重复传入：
-- user_id, min_usn_inclusive, max_usn_exclusive
-- 返回当前 Pull 范围内有更新的实体类型，顺序固定为：
-- collection -> deck -> note_type -> processing_note -> note -> card -> review_log

(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 1 AND usn >= ? AND usn < ? LIMIT 1)
UNION ALL
(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 2 AND usn >= ? AND usn < ? LIMIT 1)
UNION ALL
(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 3 AND usn >= ? AND usn < ? LIMIT 1)
UNION ALL
(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 4 AND usn >= ? AND usn < ? LIMIT 1)
UNION ALL
(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 5 AND usn >= ? AND usn < ? LIMIT 1)
UNION ALL
(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 6 AND usn >= ? AND usn < ? LIMIT 1)
UNION ALL
(SELECT entity_type FROM sync_units WHERE user_id = ? AND entity_type = 7 AND usn >= ? AND usn < ? LIMIT 1);
