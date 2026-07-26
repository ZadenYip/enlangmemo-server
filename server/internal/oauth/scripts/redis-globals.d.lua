---@meta

---@class Redis
---@field call fun(command: string, ...: any): any
---@field error_reply fun(message: string): any
redis = {}

---@class CJson
---@field decode fun(json: string): boolean, any
---@field encode fun(data: any): string
cjson = {}

---@type string[]
KEYS = {}

---@type string[]
ARGV = {}