-- =============================================================================
-- 005: 排位模式 —— users 表增加段位星级字段 + rank_settlements 结算表
-- 4 人局散台时结算：净赢家 +1 星（连胜有额外奖励），末位 -1 星（0 星保底），
-- 段位由 rank_stars 累计推导（见 backend/internal/rank）。
-- GORM AutoMigrate 也会自动添加，此文件供手动迁移使用。
-- =============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_stars INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_wins INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_draws INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_losses INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_streak INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_best_streak INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank_points INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS rank_settlements (
    id           BIGINT PRIMARY KEY AUTO_INCREMENT,
    game_id      BIGINT NOT NULL,
    user_id      BIGINT NOT NULL,
    result       VARCHAR(8) NOT NULL,
    score        INT NOT NULL,
    stars_delta  INT NOT NULL,
    bonus_stars  INT NOT NULL DEFAULT 0,
    streak_after INT NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_rank_game_user (game_id, user_id),
    INDEX idx_rank_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
