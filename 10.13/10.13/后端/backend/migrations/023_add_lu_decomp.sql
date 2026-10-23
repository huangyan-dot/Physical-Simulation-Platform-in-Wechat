-- 023_add_lu_decomp.sql - 数值分析实验批次 L17：LU 分解（Doolittle）解线性方程组（数据组）
-- 2026-10-09 新增（用户点名"继续加实验"，AskUserQuestion 选定 LU 分解，
-- 与 L13 列主元高斯消去同族对照：直接法三角分解路径）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：022（v0.21 后 L26 隐式欧拉为链尾 level 26）。本关接在解锁链第 17 位。
-- 链（v0.22）：L11 二分(1) -> ... -> L25 三次样条插值(15) -> L26 隐式欧拉法(16)
--   -> L27 LU 分解解线性方程组(17)。
--
-- 评分真值 = LU 分解路径解（后端 luDecompose Doolittle 不选主元 A=LU
-- + 前代 Ly=b + 回代 Ux=y 现算，不落库，"真值=方法输出"同全系列设计）。
-- 题库复用 linSystems（L13 同款五题，均无需选主元即可分解）：
--   sys3_basic   解 (2,3,-1)：L 乘子 [-1.5,-1,4]，U 对角 2/0.5/-1（手算闭式）；
--   sys4_tridiag 解 (1,-1,2,0.5)；sys3_diagdom 解 (1,2,3)；
--   hilbert4     病态解全 1（LU 不选主元仍可分解，SPD）；sys6_poisson 解全 1。
-- 防作弊语义：解与 L13 列主元高斯数学同源（同 L18/L16 先例：抄=算对），
-- 过程真伪由 l_col 乘子审计暴露（L 的 l21,l31,l32,... 按行序，长度 n(n-1)/2）；
-- 抄 b 向量/零向量低分不过线；自定义 A/b 换题后端重算真值防背答案；
-- 自定义矩阵主元≈0 或奇异 -> 400（不选主元 LU 会中断，与 L13 稳定性差异点）。

-- ========== 实验 27：LU 分解解线性方程组 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (27, 'lu_decomp', 'LU 分解解线性方程组', 'canvas_2d',
    JSON_OBJECT(
      'system_key', 'sys3_basic',
      'system_desc', 'A=LU 三角分解（Doolittle，不选主元）后分两步求解：前代 Ly=b 得 y，回代 Ux=y 得解。默认题 2x+y-z=8, -3x-y+2z=-11, -2x+y+2z=-3，解恰为整数 (2,3,-1)，乘子列 [-1.5,-1,4] 全手算；换题 hilbert4 观察病态、sys6_poisson 观察三对角',
      'method_hint', 'Doolittle 分解 A=LU：消去乘数 m_ik 存入 L（单位下三角），消去结果即 U（上三角）；一次分解可解多组 b（Ax=b -> LUx=b）。注意不选主元时主元≈0 分解中断（列主元高斯才稳定），题库五题均可直接分解',
      'a_note', '与 L13 列主元高斯消去同族：消去过程数学相同，LU 把乘数存下来复用',
      'pass_note', '60'),
    JSON_OBJECT(
      'method', 'lu_rel_err',
      'system_key', 'sys3_basic',
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 27：前置 L26，解锁链第 17 位（难度 3：三角分解+两段回代为新结构） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (27, 27, 'LU 分解解线性方程组', 17, 3, 26)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
