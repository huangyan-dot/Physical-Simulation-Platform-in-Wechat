-- 004_delete_extra_experiments.sql - 只保留牛顿环/示波器/单摆 3 个实验，删除其余全部
-- 幂等：可重复执行。Windows 下务必加 --default-character-set=utf8mb4。
-- 适用于已导入过旧 004（young_modulus/hall_effect/michelson）或更多批次实验的库。

USE physics_lab;

-- ========== 1. 先删成绩记录（外键引用 levels） ==========
DELETE FROM user_progresses WHERE level_id NOT IN (SELECT id FROM levels WHERE order_no <= 3);

-- ========== 2. 删关卡（保留 L1-L3） ==========
DELETE FROM levels WHERE order_no > 3;

-- ========== 3. 删实验（保留 ID 1-3：牛顿环/示波器/单摆） ==========
DELETE FROM experiments WHERE id > 3;

-- ========== 验证 ==========
-- SELECT COUNT(*) FROM experiments;  -- 应为 3
-- SELECT COUNT(*) FROM levels;        -- 应为 3
