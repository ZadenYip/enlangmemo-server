-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = request_session_id
--
-- return 1: 释放成功
-- return 2: session 不存在
-- return 3: session_id 不匹配

local sessionID = redis.call("HGET", KEYS[1], "session_id")

if sessionID == false then
  return 2
end

if sessionID ~= ARGV[1] then
  return 3
end

redis.call("DEL", KEYS[1])
return 1
