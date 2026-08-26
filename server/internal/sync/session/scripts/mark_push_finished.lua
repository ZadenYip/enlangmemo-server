-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = request_session_id
-- ARGV[2] = current_batch_seq
-- ARGV[3] = ttl seconds
--
-- return 1: 更新成功
-- return 2: session 不存在或 session 数据不完整
-- return 3: session_id 不匹配
-- return 4: batch_seq 不匹配
-- return 5: session state 不允许 finish push
-- 抛出 error: 其他错误

local STATE_PUSHING = 2
local STATE_AWAITING_FINISH = 4

local session = redis.call("HMGET", KEYS[1], "session_id", "state", "expected_batch_seq")
local sessionID = session[1]
local state = tonumber(session[2])
local expectedBatchSeq = tonumber(session[3])
local curBatchSeq = tonumber(ARGV[2])

if sessionID == false or state == nil or expectedBatchSeq == nil or curBatchSeq == nil then
  return 2
end

if sessionID ~= ARGV[1] then
  return 3
end

if state ~= STATE_PUSHING then
  return 5
end

if expectedBatchSeq ~= curBatchSeq then
  return 4
end

redis.call("HSET", KEYS[1], "state", STATE_AWAITING_FINISH)
redis.call("EXPIRE", KEYS[1], ARGV[3])

return 1
