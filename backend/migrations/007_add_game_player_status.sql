-- 007_add_game_player_status.sql
-- 玩家在台状态：active=在座 | left=已离座（自己退出 / 被台主移除）
--
-- 为什么用软删除而不是 DELETE：
--   round_submissions.game_player_id → game_players(id)   (fk_rs_player)
--   score_adjustments.from_player_id → game_players(id)   (fk_adj_from)
--   score_adjustments.to_player_id   → game_players(id)   (fk_adj_to)
--   物理删除会撞外键，且历史记分/流水会失去归属。
--
-- 说明：本表结构由程序 AutoMigrate 维护（见 backend/internal/store/store.go），
--       本文件仅作变更留档，无需手动执行。

ALTER TABLE game_players
    ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active=在座 left=已离座';

-- 离座玩家的 seat 归零，让 freeSeat / backfillPlayerSeats 能重新分配该座位
UPDATE game_players SET status = 'active' WHERE status = '' OR status IS NULL;

CREATE INDEX idx_gp_game_status ON game_players (game_id, status);
