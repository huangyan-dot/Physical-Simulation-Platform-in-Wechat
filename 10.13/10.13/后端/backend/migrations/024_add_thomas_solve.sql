-- 024_add_thomas_solve.sql - 数值分析实验批次 L18：追赶法解三对角方程组（数据组）
-- 2026-10-09 新增（用户点名"做追赶法"，L27 LU 分解的三对角特例：
-- Crout 消元"追"+回代"赶"，O(n) 存储/O(n) 计算）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：023（v0.22 后 L27 LU 分解为链尾 level 27）。本关接在解锁链第 18 位。
-- 链（v0.23）：L11 二分(1) -> ... -> L26 隐式欧拉法(16) -> L27 LU 分解解线性方程组(17)
--   -> L28 追赶法解三对角方程组(18)。
--
-- 评分真值 = 追赶法解（后端 thomasChase 现算，不落库，"真值=方法输出"同全系列）。
-- 新题库 triSystems（三对角紧凑 a/b/c/d 存储，四题均对角占优或对称正定，
-- 追赶法不选主元必安全）：
--   tri1_basic  手算例题 4 阶：m=(0.5,0.8,0.3125)，beta=(2,2.5,3.2,4.6875)，
--               d'=(4,8,13.6,18.75)，解恰 (1,2,3,4)；
--   tri2_poisson 一维泊松差分 5 内点（对称正定）：beta=(2,3/2,4/3,5/4,6/5)，解全 1；
--   tri3_small  入门 3 阶（严格对角占优）：m=(1/4,2/7)，解恰 (1,1,1)；
--   tri4_spline 三次样条三弯矩系数阵 6 阶（L25 呼应：[2,1; 1,4,1; ...; 1,2]），解全 1。
-- 防作弊语义：解与 L13 列主元高斯 / L27 LU 数学同源（同 L18/L16 先例：抄=算对），
-- 过程真伪由 m（乘子 m2..mn，长 n-1）与 beta（新主对角，长 n）双审计暴露；
-- 抄 d 向量/零向量低分不过线；换题后端重算真值防背答案；
-- 自定义三对角 beta≈0 -> 400（追赶法不选主元会中断——适用条件：对角占优或
-- 对称正定，与 L13 列主元高斯的稳定性差异教学点）。

-- ========== 实验 28：追赶法解三对角方程组 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (28, 'thomas_solve', '追赶法解三对角方程组', 'canvas_2d',
    JSON_OBJECT(
      'system_key', 'tri1_basic',
      'system_desc', '三对角方程组的专用解法（LU 分解的特例）：追（Crout 消元）依次算乘数 m 与新主对角 beta 并更新右端，赶（回代）自最后一个未知量倒推解。默认题主对角 (2,3,4,5)、上下对角见页面，解恰 (1,2,3,4)，m 与 beta 全手算；换题 tri2_poisson（对称正定）、tri4_spline（L25 三弯矩阵）',
      'method_hint', '追：beta_1=b_1，m_i=a_{i-1}/beta_{i-1}，beta_i=b_i-m_i*c_{i-1}，d_i<-d_i-m_i*d_{i-1}；赶：x_n=d_n/beta_n，x_i=(d_i-c_i*x_{i+1})/beta_i。仅存三条对角线，O(n) 计算量——大型三对角阵（热传导/样条）的标准解法；不选主元，对角占优或对称正定时必安全',
      'a_note', '与 L27 LU 分解同族：三对角时 L/U 只各剩一条对角线，追赶法即其紧凑形式',
      'pass_note', '60'),
    JSON_OBJECT(
      'method', 'thomas_rel_err',
      'system_key', 'tri1_basic',
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 28：前置 L27，解锁链第 18 位（难度 2：公式直接、双审计字段） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (28, 28, '追赶法解三对角方程组', 18, 2, 27)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
