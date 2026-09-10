-- =============================================================================
-- 006: 换位申请 —— seat_swap_requests 表（PRD §8.7）
-- 长按空位：即时换座（swap_seat，无需落表）
-- 长按他人座位：生成 pending 申请，对方确认后互换两个已占用座位
-- GORM AutoMigrate 也会自动建表，此文件供手动迁移使用。
-- =============================================================================

CREATE TABLE IF NOT EXISTS seat_swap_requests (
    id             BIGINT PRIMARY KEY AUTO_INCREMENT,
    game_id        BIGINT NOT NULL,
    from_player_id BIGINT NOT NULL,
    to_player_id   BIGINT NOT NULL,
    from_seat      INT NOT NULL,
    to_seat        INT NOT NULL,
    status         VARCHAR(16) NOT NULL DEFAULT 'pending',
    expires_at     DATETIME NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at    DATETIME NULL,
    INDEX idx_swap_game (game_id),
    INDEX idx_swap_to (to_player_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
