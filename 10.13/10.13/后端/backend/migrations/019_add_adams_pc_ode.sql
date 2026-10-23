-- 019_add_adams_pc_ode.sql - 数值分析实验批次 L13：阿当姆斯预报-校正法（数据组）
-- 2026-10-01 新增（线性多步法，与 L20/L21/L22 单步法构成方法族对照）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：018（v0.17 后 L22 龙格-库塔为链尾 level 22）。本关接在解锁链第 13 位。
-- 链（v0.18）：L11 二分(1) -> ... -> L20 显式欧拉(10) -> L21 改进欧拉(11) -> L22 龙格-库塔(12) -> L23 阿当姆斯预报-校正(13)。
--
-- 评分真值 = 方法输出 y_N（后端按同一 f、初值、区间、步长现算 adamsPCColumn 不落库，
-- "真值=方法输出"同 L7/L9/L20/L21/L22 设计；精确解闭式记审计 ref_Y 不打分）。
-- 方法：AB4 外推预报 + AM4 内插校正一次（PECE 模式），
-- 表头 y_1..y_3 用 RK4 启动（与 L22 同式，表头同阶精度）；
-- n 须 4~1024（3 步表头 + 至少 1 步阿当姆斯）。
-- 题库复用 odeProblems（exp_growth / decay_sq / stiff_decay，均 [0,1] 默认 n=4）。
-- target 默认 exp_growth n=4：表头 y_1..y_3 = R(1/4)^i
-- （R(h)=1+h+h^2/2+h^3/6+h^4/24），第 4 步 AB4-AM4 终值 2.7182259833。
-- 防作弊语义：四阶法收敛快，抄精确解 e 约 93 分、抄 L22 默认答案 65/24 约 72 分，
-- 均过线但非满分（同为四阶法数学上难分，过程真伪由 y_col 审计暴露，L19/L22 同款
-- 语义）；抄 L21 改进欧拉答案 2.640625 约 49 分、抄 L20 显式答案 2.44140625 约
-- 30 分均不过线；stiff_decay（h*lamda=-2.5）多步法绝对稳定区间远小于 RK4，
-- 抄精确解 e^{-10} 直接 0 分，抄 RK4 n=4 表头答案 0.1768 约 9 分也被拦。

-- ========== 实验 23：阿当姆斯预报-校正法 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (23, 'adams_pc_ode', '阿当姆斯预报-校正法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'exp_growth',
      'problem_desc', 'y''=y, y(0)=1 的阿当姆斯线性多步法：前 3 步用 RK4 启动做表头，第 4 步起 AB4 外推预报 + AM4 内插校正一次（PECE），求 y(1)（默认 n=4，h=0.25）',
      'func_desc', 'f(x, y) = y（AB4-AM4 全局截断误差 O(h^4)，与 L20/L21/L22 单步法构成方法族对照；多步法一次算出 4 个历史斜率的加权组合）',
      'a', 0,
      'b', 1,
      'y0', 1,
      'n', 4,
      'method_hint', '表头 RK4：y_{i+1} = y_i + h/6*(K1+2K2+2K3+K4)；预报 AB4：y^p_{n+1} = y_n + h/24*(55f_n - 59f_{n-1} + 37f_{n-2} - 9f_{n-3})；校正 AM4：y_{n+1} = y_n + h/24*(9f(x_{n+1}, y^p) + 19f_n - 5f_{n-1} + f_{n-2})，只校一次'
    ),
    JSON_OBJECT(
      'method', 'adams_rel_err',
      'problem_key', 'exp_growth',
      'n', 4,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 23：前置 L22，解锁链第 13 位（难度 4，链上最高） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (23, 23, '阿当姆斯预报-校正法', 13, 4, 22)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
