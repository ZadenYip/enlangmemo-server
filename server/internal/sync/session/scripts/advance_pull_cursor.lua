-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = request_session_id
-- ARGV[2] = next_sync_cursor_usn
-- ARGV[3] = ttl seconds
--
-- return 1: 更新成功
-- return 2: session 不存在或 session 数据不完整
-- return 3: session_id 不匹配
-- 抛出 error: 其他错误

local sessionID = redis.call("HGET", KEYS[1], "session_id")
local nextSyncCursorUSN = tonumber(ARGV[2])

if sessionID == false or nextSyncCursorUSN == nil then
  return 2
end

if sessionID ~= ARGV[1] then
  return 3
end

redis.call("HSET", KEYS[1], "sync_cursor_usn", nextSyncCursorUSN)
redis.call("EXPIRE", KEYS[1], ARGV[3])

return 1
