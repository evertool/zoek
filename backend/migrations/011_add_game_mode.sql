-- 011: games 增加 mode 字段（round 锁局模式 / free 自由模式）
-- 默认 round（推荐锁局模式）
ALTER TABLE games
  ADD COLUMN IF NOT EXISTS mode VARCHAR(16) NOT NULL DEFAULT 'round' AFTER name;
