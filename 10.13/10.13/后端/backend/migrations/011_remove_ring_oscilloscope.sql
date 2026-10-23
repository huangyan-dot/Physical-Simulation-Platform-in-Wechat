-- 011_remove_ring_oscilloscope.sql
-- v0.10（2026-09-09）：删除物理实验批次（牛顿环 id=1、示波器 id=2）的全部数据。
-- 项目转向数值分析实验链，物理实验下线；解锁链头改为 L11 二分法。
-- 幂等：所有语句按主键/条件执行，重复运行无副作用。

USE physics_lab;

-- 1. 先删引用这两关的进度与操作日志
DELETE FROM user_progresses WHERE level_id IN (1, 2);
DELETE FROM operation_logs WHERE level_id IN (1, 2);

-- 2. 解锁链重排：L11 二分法原先挂在示波器(id=2)之后，改为链头（无前置）
UPDATE levels SET prereq_level_id = NULL WHERE id = 11 AND prereq_level_id = 2;

-- 3. 关卡顺序号重排（物理实验占 1-4，数值分析 11-14 顶替为 1-4）
UPDATE levels SET order_no = 1 WHERE id = 11;
UPDATE levels SET order_no = 2 WHERE id = 12;
UPDATE levels SET order_no = 3 WHERE id = 13;
UPDATE levels SET order_no = 4 WHERE id = 14;

-- 4. 删除关卡与实验本体
DELETE FROM levels WHERE id IN (1, 2);
DELETE FROM experiments WHERE id IN (1, 2);
