-- 009: games 增加 duration_minutes 落库字段
-- 散台（手动/系统自动）时一次性写入 = ended_at - started_at（分钟），
-- 记录页等读侧直接取字段，不再每次请求现算。
-- 服务启动 AutoMigrate 会自动加列；此文件同时负责存量回填。

ALTER TABLE games
  ADD COLUMN IF NOT EXISTS duration_minutes INT NOT NULL DEFAULT 0 AFTER settlement_updated_at;

-- 回填存量已结束牌局
UPDATE games
SET duration_minutes = GREATEST(TIMESTAMPDIFF(MINUTE, started_at, ended_at), 0)
WHERE status = 'ended'
  AND started_at IS NOT NULL
  AND ended_at IS NOT NULL
  AND duration_minutes = 0;

-- 纠正旧版自动散台的存量数据：ended_at 曾写的是超时扫描触发时刻（最后一笔账 + 5h）。
-- 指纹：ended_at - 最后一笔账 >= 5h（295 分钟留容差），命中的把 ended_at 纠正为
-- 最后一笔账时间（不低于 started_at），随后一并重算 duration_minutes。
-- 可重复执行；手动散台的牌局结束时间与最后一笔账接近，不会被误伤。
UPDATE games g
JOIN (
  SELECT g2.id,
         GREATEST(
           COALESCE((SELECT MAX(a.created_at) FROM score_adjustments a WHERE a.game_id = g2.id), g2.created_at),
           COALESCE((SELECT MAX(rs.updated_at) FROM round_submissions rs JOIN rounds r ON rs.round_id = r.id WHERE r.game_id = g2.id), g2.created_at)
         ) AS last_entry
  FROM games g2 WHERE g2.status = 'ended'
) le ON le.id = g.id
SET g.ended_at = GREATEST(le.last_entry, COALESCE(g.started_at, le.last_entry)),
    g.duration_minutes = GREATEST(
      TIMESTAMPDIFF(MINUTE, g.started_at, GREATEST(le.last_entry, COALESCE(g.started_at, le.last_entry))), 0)
WHERE g.started_at IS NOT NULL
  AND g.ended_at IS NOT NULL
  AND g.ended_at > le.last_entry + INTERVAL 295 MINUTE;
