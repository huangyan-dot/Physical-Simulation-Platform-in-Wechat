-- 025_add_cholesky_solve.sql - 数值分析实验批次 L19：平方根法解对称正定方程组（数据组）
-- 2026-10-11 新增（直接法收官：L27 LU 分解的对称正定特例 A=LLᵀ，
-- Cholesky 按行分解+前代 Ly=b+回代 Lᵀx=y，计算量约为 LU 一半且永不选主元）。
-- 幂等：可重复执行。Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，
-- 否则中文报错 1366。
--
-- 前置：024（v0.23 后 L28 追赶法为链尾 level 28）。本关接在解锁链第 19 位。
-- 链（v0.24）：L11 二分(1) -> ... -> L27 LU 分解解线性方程组(17)
--   -> L28 追赶法解三对角方程组(18) -> L29 平方根法解对称正定方程组(19)。
--
-- 评分真值 = 平方根法解（后端 choleskySolve 现算，不落库，"真值=方法输出"同全系列）。
-- 新题库 spdSystems（对称正定四题，L/y/x 全整数，课堂手算友好）：
--   spd1_2x2       入门 2 阶：L=[[2,0],[1,2]]，y=(3,2)，解恰 (1,1)；
--   spd2_min3      min(i,j) 阵 3 阶（默认题）：L=下三角全 1，y=(6,5,3)，解恰 (1,2,3)；
--   spd3_tridiag3  三对角 SPD（L28 呼应）：L=[[2,0,0],[1,2,0],[0,1,3]]，y=(3,3,3)，解恰 (1,1,1)；
--   spd4_min4      min(i,j) 阵 4 阶：L=下三角全 1，y=(4,3,2,1)，解全 1。
-- 防作弊语义：解与 L13 列主元高斯 / L27 LU / L28 追赶数学同源（同 L18/L16 先例：
-- 抄=算对），过程真伪由 L（n 阶方阵，对照取下三角含主对角 n(n+1)/2 个元）与
-- y（前代中间向量，长 n）双审计暴露；抄 b 向量/零向量低分不过线；
-- 换题后端重算真值防背答案；自定义 A 非对称或根号内≤0 -> 400
-- （平方根法只适用于对称正定阵——正是与 L13 列主元高斯的差异教学点）。

-- ========== 实验 29：平方根法解对称正定方程组 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (29, 'cholesky_solve', '平方根法解对称正定方程组', 'canvas_2d',
    JSON_OBJECT(
      'system_key', 'spd2_min3',
      'system_desc', '对称正定方程组的专用解法（LU 分解的特例）：A = L·Lᵀ，按行依次算出下三角因子 L（主对角全正），再前代 Ly=b、回代 Lᵀx=y。默认题 min(i,j) 阵 3 阶，L 恰为下三角全 1，y=(6,5,3)，解恰 (1,2,3) 全手算；换题 spd1 入门 2 阶、spd3 三对角（L28 呼应）、spd4 四阶',
      'method_hint', '分解：l_ii=sqrt(a_ii-Σ_{k<i} l_ik²)，l_ij=(a_ij-Σ_{k<j} l_ik·l_jk)/l_jj（j<i）；前代 y_i=(b_i-Σ_{k<i} l_ik·y_k)/l_ii；回代 x_i=(y_i-Σ_{k>i} l_ki·x_k)/l_ii。对称正定保证根号内恒正、永不选主元，计算量约为 LU 一半——协方差矩阵/有限元刚度阵的标准解法',
      'a_note', '与 L27 LU 分解同族：对称正定时 U=Lᵀ 只需存一个三角阵；L28 追赶法又是本法的三对角特例',
      'pass_note', '60'),
    JSON_OBJECT(
      'method', 'cholesky_rel_err',
      'system_key', 'spd2_min3',
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 29：前置 L28，解锁链第 19 位（难度 2：公式直接、双审计字段） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (29, 29, '平方根法解对称正定方程组', 19, 2, 28)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
