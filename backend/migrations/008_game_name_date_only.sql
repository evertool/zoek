-- 008_game_name_date_only.sql
-- 牌局标题去掉固定的「得闲开台」前缀，只保留「M月D日」。
--
-- 背景：
--   开台时（POST /api/v1/games 不传 name）由后端自动生成台名，
--   旧格式 =「得闲开台 9月14日」，新格式 =「9月14日」
--   （见 backend/internal/handler/game.go 的 defaultGameName）。
--
-- 说明：
--   本文件**仅做数据订正**，表结构由程序 AutoMigrate 维护（见 internal/store/store.go），
--   不需要建表语句。与 003~007 一样，本文件不会被程序自动执行，
--   只在需要让「旧牌局」也显示新格式时，在服务器上手动跑一次：
--
--     mysql -u root -p zoek < 008_game_name_date_only.sql
--
--   不跑也不影响使用：新开的台已经是新格式，旧牌局仍显示带前缀的老标题。
--
-- 安全性：前端开台时 name 恒传空串，台名一律由后端生成，
--         因此不存在「用户自取的名字恰好以『得闲开台 』开头」的情况，可放心裁剪。

UPDATE games
SET name = TRIM(SUBSTRING(name, CHAR_LENGTH('得闲开台 ') + 1))
WHERE name LIKE '得闲开台 %'
  AND TRIM(SUBSTRING(name, CHAR_LENGTH('得闲开台 ') + 1)) <> '';
