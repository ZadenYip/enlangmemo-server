-- return 0: token 不存在
-- return 1: token 撤销成功
-- return 2: client_id 不匹配
local token_info = redis.call("GET", KEYS[1])

if not token_info then
    return 0
end

local ok, decoded = pcall(cjson.decode, token_info)

if not ok then
    return redis.error_reply("ERR_CORRUPTED_TOKEN_JSON")
end

if type(decoded["client_id"]) ~= "string" then
    return redis.error_reply("ERR_CORRUPTED_TOKEN_JSON")
end

if decoded["client_id"] ~= ARGV[1] then
    return 2
end

redis.call("DEL", KEYS[1])
return 1
