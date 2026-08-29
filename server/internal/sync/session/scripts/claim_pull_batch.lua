-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = request_session_id
-- ARGV[2] = current_batch_seq
-- ARGV[3] = ttl seconds
--
-- return {1, sync_cursor_usn, server_sync_cursor_usn_at_handshake, pull_entity_queue}: 校验成功，并推进了 batch_seq
-- return {2, 0, 0, ""}: session 不存在或 session 数据不完整
-- return {3, 0, 0, ""}: session_id 不匹配
-- return {4, 0, 0, ""}: batch_seq 不匹配
-- return {5, 0, 0, ""}: session state 不允许 Pull
-- 抛出 error: 其他错误

local STATE_PULLING = 1

local session = redis.call(
  "HMGET",
  KEYS[1],
  "session_id",
  "state",
  "expected_batch_seq",
  "sync_cursor_usn",
  "server_sync_cursor_usn_at_handshake",
  "pull_entity_queue"
)
local sessionID = session[1]
local state = tonumber(session[2])
local expectedBatchSeq = tonumber(session[3])
local syncCursorUSN = tonumber(session[4])
local serverSyncCursorUSNAtHandshake = tonumber(session[5])
local pullEntityQueue = session[6]
local curBatchSeq = tonumber(ARGV[2])

if sessionID == false or state == nil or expectedBatchSeq == nil or syncCursorUSN == nil or serverSyncCursorUSNAtHandshake == nil or pullEntityQueue == false or curBatchSeq == nil then
  return {2, 0, 0, ""}
end

if sessionID ~= ARGV[1] then
  return {3, 0, 0, ""}
end

if state ~= STATE_PULLING then
  return {5, 0, 0, ""}
end

if expectedBatchSeq ~= curBatchSeq then
  return {4, 0, 0, ""}
end

redis.call("HSET", KEYS[1], "expected_batch_seq", curBatchSeq + 1)
redis.call("EXPIRE", KEYS[1], ARGV[3])

return {1, syncCursorUSN, serverSyncCursorUSNAtHandshake, pullEntityQueue}
