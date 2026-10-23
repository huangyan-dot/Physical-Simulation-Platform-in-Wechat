-- 021_add_cubic_spline.sql - 数值分析实验批次 L15：三次样条插值（数据组）
-- 2026-10-03 新增（数值链收官后新篇：插值系列补最后经典，难度 4）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：020（v0.19 后 L24 最小二乘拟合为链尾 level 24）。本关接在解锁链第 15 位。
-- 链（v0.20）：L11 二分(1) -> ... -> L23 阿当姆斯预报-校正(13) -> L24 最小二乘拟合(14)
--   -> L25 三次样条插值(15)。
--
-- 评分真值 = 自然边界三次样条 S(x_eval)（后端按同一节点、同一求值点现算
-- splineMoments + splineEvalAt 不落库，"真值=方法输出"同 L7/L9/L16/L20-L24 设计）。
-- 三弯矩方程（自然边界 M_0=M_{n-1}=0）：
--   mu_i*M_{i-1} + 2*M_i + lam_i*M_{i+1} = d_i（i=1..n-2），
--   mu_i = h_i/(h_i+h_{i+1})，lam_i = h_{i+1}/(h_i+h_{i+1})，
--   d_i  = 6*[(y_{i+1}-y_i)/h_{i+1} - (y_i-y_{i-1})/h_i]/(h_i+h_{i+1})，
-- 三对角追赶法求解；段 [x_k,x_{k+1}] 上 S(t) = y_k + B*w + C*w^2 + D*w^3（w=t-x_k），
--   B = (y_{k+1}-y_k)/h - h*(2M_k+M_{k+1})/6，C = M_k/2，D = (M_{k+1}-M_k)/(6h)。
-- 题库 splineProblems（前后端逐字同步）：
--   dist_4 位移观测数据 4 节点（不等距 h=1,1,2），三弯矩手算闭式
--     M1=15/23、M2=78/23，真值 S(3)=559/46（手算验证友好）；
--   sin_5 sin 五等距节点（对称 M1=M3），真值 S(pi/6) 约 0.49974——
--     光滑函数上样条逼近极好，抄 sin(pi/6)=0.5 约 83 分过线但非满分
--     （过程真伪由 m_col 审计暴露，同 L19 抄 I / L22 抄 e 语义）；
--   runge_5 龙格函数五等距节点（与 L16 拉格朗日同节点同求值点三法对照）：
--     真值 S(0.8) 约 -0.036340，抄拉格朗日 P(0.8)=-0.379310、抄 f(0.8)=1/17、
--     抄观测值全部 0 分——样条治龙格的主场题（教学点：自然边界端段仍下弯，
--     clamped 边界更准但需导数信息）。
-- 防作弊语义（默认曲线 pass_score=60）：dist_4 抄拉格朗日 P(3)=11.5 约 39 分、
-- 抄线性内插 13 约 35 分、抄观测全 0 分，只有真解三弯矩才过线；
-- 样条不外推：x_eval 须在节点区间内（与 L16 拉格朗日可外推不同）。

-- ========== 实验 25：三次样条插值 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (25, 'cubic_spline', '三次样条插值', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'dist_4',
      'problem_desc', '位移观测数据 (0,0),(1,2.5),(2,6),(4,20)：先列 h_i 与 mu/lambda/d 组装三弯矩方程（不等距 h=1,1,2），追赶法解出弯矩 M1=15/23、M2=78/23，再按段公式求 S(3)=559/46；可换题 sin 五等距节点 / 龙格函数（与 L16 拉格朗日同题对照）',
      'func_desc', '三次样条：过所有节点的分段三次多项式，节点处一阶二阶导连续；自然边界 M_0=M_{n-1}=0 下由三弯矩方程 mu_i*M_{i-1}+2M_i+lam_i*M_{i+1}=d_i 解出弯矩列，段上 S(t)=y_k+B*w+C*w^2+D*w^3',
      'x_eval', 3,
      'method_hint', '三弯矩（自然边界）：mu_i=h_i/(h_i+h_{i+1})，lam_i=1-mu_i，d_i=6*[(y_{i+1}-y_i)/h_{i+1}-(y_i-y_{i-1})/h_i]/(h_i+h_{i+1})；追赶法解 M 后按段公式求 S(x_eval)；样条不外推，x_eval 须在区间内'
    ),
    JSON_OBJECT(
      'method', 'spline_rel_err',
      'problem_key', 'dist_4',
      'x_eval', 3,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 25：前置 L24，解锁链第 15 位（难度 4，与 L22/L23 并列最高） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (25, 25, '三次样条插值', 15, 4, 24)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
