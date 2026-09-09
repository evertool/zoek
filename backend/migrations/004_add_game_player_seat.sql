-- =============================================================================
-- 004: 给 game_players 表添加 seat 字段（座位号 1-4，東南西北）
-- 0 表示旧数据未分配，服务启动时 AutoMigrate 也会按加入顺序自动回填。
-- 此文件供手动迁移使用。
-- =============================================================================

ALTER TABLE game_players ADD COLUMN seat SMALLINT NOT NULL DEFAULT 0 AFTER role;

-- 按加入顺序为存量玩家回填座位号
UPDATE game_players gp
JOIN (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY game_id ORDER BY joined_at ASC, id ASC) AS rn
    FROM game_players
) t ON gp.id = t.id
SET gp.seat = t.rn
WHERE gp.seat = 0;
