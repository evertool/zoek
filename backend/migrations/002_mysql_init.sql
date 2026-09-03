-- =============================================================================
-- zoekt MySQL 版初始数据库迁移 (PRD §7.2 + §7.5)
-- 用于生产环境 MySQL，与 SQLite 版 001_init.sql 结构一致
-- 用法: mysql -h <host> -u <user> -p <db_name> < migrations/002_mysql_init.sql
-- =============================================================================

CREATE TABLE IF NOT EXISTS users (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    openid      VARCHAR(64) NOT NULL UNIQUE,
    nickname    VARCHAR(32) NOT NULL,
    avatar_url  VARCHAR(512),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS games (
    id                    BIGINT PRIMARY KEY AUTO_INCREMENT,
    creator_id            BIGINT NOT NULL,
    name                  VARCHAR(64) NOT NULL DEFAULT '未命名牌局',
    status                VARCHAR(16) NOT NULL DEFAULT 'forming',
    invite_token_hash     VARCHAR(128) NOT NULL UNIQUE,
    join_expires_at       DATETIME NULL,
    members_locked        TINYINT(1) NOT NULL DEFAULT 0,
    started_at            DATETIME NULL,
    ended_at              DATETIME NULL,
    settlement_updated_at DATETIME NULL,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    version               INT NOT NULL DEFAULT 1,
    INDEX idx_creator_status (creator_id, status),
    CONSTRAINT fk_games_creator FOREIGN KEY (creator_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS game_players (
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    game_id           BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    nickname_snapshot VARCHAR(32) NOT NULL,
    role              VARCHAR(16) NOT NULL DEFAULT 'player',
    joined_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_game_user (game_id, user_id),
    INDEX idx_game (game_id),
    INDEX idx_user (user_id),
    CONSTRAINT fk_gp_game FOREIGN KEY (game_id) REFERENCES games(id),
    CONSTRAINT fk_gp_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS rounds (
    id                 BIGINT PRIMARY KEY AUTO_INCREMENT,
    game_id            BIGINT NOT NULL,
    round_number       INT NOT NULL,
    status             VARCHAR(16) NOT NULL DEFAULT 'open',
    review_started_at  DATETIME NULL,
    locked_at          DATETIME NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_game_round (game_id, round_number),
    INDEX idx_game_status (game_id, status),
    CONSTRAINT fk_rounds_game FOREIGN KEY (game_id) REFERENCES games(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS round_submissions (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    round_id        BIGINT NOT NULL,
    game_player_id  BIGINT NOT NULL,
    score           INT NOT NULL,
    request_id      VARCHAR(64) NOT NULL,
    submitted_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_round_player (round_id, game_player_id),
    UNIQUE KEY uk_round_request (round_id, request_id),
    INDEX idx_round (round_id),
    CONSTRAINT fk_rs_round FOREIGN KEY (round_id) REFERENCES rounds(id),
    CONSTRAINT fk_rs_player FOREIGN KEY (game_player_id) REFERENCES game_players(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS score_adjustments (
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    game_id           BIGINT NOT NULL,
    round_id          BIGINT NOT NULL,
    from_player_id    BIGINT NOT NULL,
    to_player_id      BIGINT NOT NULL,
    adjustment_type   VARCHAR(16) NOT NULL,
    amount            INT NOT NULL,
    reason            VARCHAR(256),
    proposed_by       BIGINT NOT NULL,
    status            VARCHAR(16) NOT NULL DEFAULT 'pending',
    request_id        VARCHAR(64) NOT NULL,
    expires_at        DATETIME NOT NULL,
    resolved_by       BIGINT NULL,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at       DATETIME NULL,
    UNIQUE KEY uk_adjustment_request (game_id, request_id),
    INDEX idx_adjustment_game (game_id, status),
    INDEX idx_adjustment_receiver (to_player_id, status),
    CONSTRAINT fk_adj_game FOREIGN KEY (game_id) REFERENCES games(id),
    CONSTRAINT fk_adj_round FOREIGN KEY (round_id) REFERENCES rounds(id),
    CONSTRAINT fk_adj_from FOREIGN KEY (from_player_id) REFERENCES game_players(id),
    CONSTRAINT fk_adj_to FOREIGN KEY (to_player_id) REFERENCES game_players(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
