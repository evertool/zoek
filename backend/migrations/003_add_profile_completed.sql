-- =============================================================================
-- 003: 给 users 表添加 profile_completed 字段
-- 用于标记用户是否已完善资料（昵称+持久化头像）
-- GORM AutoMigrate 也会自动添加，此文件供手动迁移使用。
-- =============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_completed TINYINT(1) NOT NULL DEFAULT 0 AFTER avatar_url;

-- 将已有昵称+持久化头像的用户标记为已完成
UPDATE users SET profile_completed = 1
WHERE nickname != ''
  AND (avatar_url LIKE 'data:image%' OR avatar_url LIKE 'https://%');
