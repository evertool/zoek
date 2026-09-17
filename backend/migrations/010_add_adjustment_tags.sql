-- 010: score_adjustments 增加 tags 落库字段
-- 给分标签（自摸/明杠/暗杠/杠爆/抢杠）可多选，落库为英文 code 的逗号串，
-- 展示文案由端上 utils/score-tags.js 映射（后端不存中文，避免改名要洗数据）。
-- 服务启动 AutoMigrate 会自动加列；此文件供手动迁移使用（可重复执行）。
-- 存量数据无需回填：NULL / 空串都按「无标签」处理。

ALTER TABLE score_adjustments
  ADD COLUMN IF NOT EXISTS tags VARCHAR(128) NULL AFTER reason;
