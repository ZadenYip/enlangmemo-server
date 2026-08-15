-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = request_session_id
-- ARGV[2] = TTL seconds
--
-- return 1: 确认当前 session 允许 finish
-- return 2: session 不存在
-- return 3: session_id 不匹配
-- return 4: session 当前 state 不允许 finish

local STATE_AWAITING_PUSH_OR_FINISH = 3
local STATE_AWAITING_FINISH = 4

local session = redis.call("HMGET", KEYS[1], "session_id", "state")
local sessionID = session[1]
local state = tonumber(session[2])

if sessionID == false or state == nil then
    return 2
end

if sessionID ~= ARGV[1] then
    return 3
end

if state ~= STATE_AWAITING_FINISH and state ~= STATE_AWAITING_PUSH_OR_FINISH then
    return 4
end

redis.call("EXPIRE", KEYS[1], ARGV[2])
return 1
