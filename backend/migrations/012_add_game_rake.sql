-- 012_add_game_rake.sql
-- 为锁局模式增加抽水配置与进度字段

ALTER TABLE games
  ADD COLUMN rake_target INT NOT NULL DEFAULT 0 COMMENT '抽水目标总分数（0表示不抽水）',
  ADD COLUMN rake_per_round INT NOT NULL DEFAULT 2 COMMENT '每局抽水分数（默认2分）',
  ADD COLUMN rake_collected INT NOT NULL DEFAULT 0 COMMENT '累计已抽水分数';

ALTER TABLE rounds
  ADD COLUMN rake_amount INT NOT NULL DEFAULT 0 COMMENT '本局实际抽取的总分数';
