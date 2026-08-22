-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = request_session_id
-- ARGV[2] = current_batch_seq
-- ARGV[3] = change_count
-- ARGV[4] = ttl seconds
--
-- return {1, assigned_start_usn}: 更新成功，并为当前 batch 预留 [assigned_start_usn, assigned_start_usn + change_count) 这一段 usn
-- return {2, 0}: session 不存在或 session 数据不完整
-- return {3, 0}: session_id 不匹配
-- return {4, 0}: batch_seq 不匹配
-- return {5, 0}: session state 不允许推进 Push
-- 抛出 error: 其他错误

local STATE_PUSHING = 2
local STATE_AWAITING_PUSH_OR_FINISH = 3

local session = redis.call("HMGET", KEYS[1], "session_id", "state", "expected_batch_seq", "sync_cursor_usn")
local sessionID = session[1]
local state = tonumber(session[2])
local expectedBatchSeq = tonumber(session[3])
local syncCursorUSN = tonumber(session[4])
local curBatchSeq = tonumber(ARGV[2])
local changeCount = tonumber(ARGV[3])

if sessionID == false or state == nil or expectedBatchSeq == nil or syncCursorUSN == nil or curBatchSeq == nil or changeCount == nil or changeCount <= 0 then
  return {2, 0}
end

if sessionID ~= ARGV[1] then
  return {3, 0}
end

if state == STATE_PUSHING then
  if expectedBatchSeq ~= curBatchSeq then
    return {4, 0}
  end
elseif state == STATE_AWAITING_PUSH_OR_FINISH then
  if expectedBatchSeq ~= 1 or curBatchSeq ~= 1 then
    return {4, 0}
  end
  redis.call("HSET", KEYS[1], "state", STATE_PUSHING)
else
  return {5, 0}
end

local assignedStartUSN = syncCursorUSN
local nextSyncCursorUSN = assignedStartUSN + changeCount
redis.call("HSET", KEYS[1], "expected_batch_seq", curBatchSeq + 1, "sync_cursor_usn", nextSyncCursorUSN)
redis.call("EXPIRE", KEYS[1], ARGV[4])

return {1, assignedStartUSN}
