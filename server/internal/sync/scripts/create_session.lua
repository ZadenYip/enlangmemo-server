-- KEYS[1] = sync:{userID}:sync_lock
-- ARGV[1] = user_id
-- ARGV[2] = state
-- ARGV[3] = expected_batch_seq
-- ARGV[4] = sync_cursor_usn
-- ARGV[5] = session_id
-- ARGV[6] = client_sync_cursor_usn_at_handshake
-- ARGV[7] = server_sync_cursor_usn_at_handshake
-- ARGV[8] = device_id
-- ARGV[9] = ttl seconds
--
-- return 0: session 已存在
-- return 1: session 创建成功
-- 抛出 error: 其他错误

local sessionKey = KEYS[1]

if redis.call("EXISTS", sessionKey) == 1 then
  return 0
end

redis.call("HSET", sessionKey,
  "user_id", ARGV[1],
  "state", ARGV[2],
  "expected_batch_seq", ARGV[3],
  "sync_cursor_usn", ARGV[4],
  "session_id", ARGV[5],
  "client_sync_cursor_usn_at_handshake", ARGV[6],
  "server_sync_cursor_usn_at_handshake", ARGV[7],
  "device_id", ARGV[8]
)

redis.call("EXPIRE", sessionKey, ARGV[9])

return 1
