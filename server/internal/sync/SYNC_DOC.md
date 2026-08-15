# 实现

本文档主要描述具体实现，如果是要查看同步协议上的设计见：https://github.com/ZadenYip/enlangmemo-sync-api/blob/main/proto/enlangmemo/sync/v1/SYNC_DOC.md。

## Redis 同步会话

同步会话存储在 `sync:{userID}:sync_lock` hash 中，并设置 60 秒TTL。`expected_batch_seq` 和 `sync_cursor_usn` 不能“先读 session，再写入下一个 batch 序号 / cursor”两步更新，因为会导致竞态条件：客户端重发同一个 batch 时，服务端可能重复执行事务插入浪费性能。

所以 Push 阶段在数据库事务之前使用脚本 `claim_push_batch.lua` 原子完成 session 状态校验、领取当前 batch 的 `assigned_usn`、推进 batch seq、推进 sync cursor usn 和续期。脚本成功时返回 `assigned_usn`，让后续 Push 逻辑可以直接拿这个 USN 绑定本次 batch 写入；失败时只返回状态码，`assigned_usn` 固定为 0，避免和成功结果混在一起。`claim_push_batch.lua` 成功后，同一个 batch slot 已经被消耗；如果后续数据库事务失败，客户端不重试同一个 batch，而是结束本轮同步，下一次重新 Handshake。

每个 Push batch 成功落库时，服务端在同一个 MySQL 事务内写入实体变更、`sync_units`，并推进 `collections.sync_cursor_usn = assigned_usn + 1`。最后一个 Push batch 成功落库后，再使用 `mark_push_finished.lua` 将 session 状态标记为 `AWAITING_FINISH`，避免在数据库事务成功前提前允许 FinishSync。

Push 请求的 `changes` 非空由 Connect validate 中间件根据 proto 约束保证。服务端领取 batch 后，在应用变更的局部路径上做结构校验：每条 `SyncChange` 不得为空，`usn` 必须为 `-1`；`UPSERT` 必须携带与 `entity_type` 匹配的 payload；`DELETE` 必须携带 `deleted_at` 且不得携带 payload；`op`、`entity_type` 必须是服务端支持的组合。结构校验失败会返回 `InvalidArgument`，但由于 batch 已经被领取，客户端仍不重试同一个 batch。冲突处理、实体依赖顺序和 payload 内业务字段合法性由客户端负责。
