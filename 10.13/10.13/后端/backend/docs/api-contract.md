# 接口契约（API Contract）v0.24

> 前后端并行开发的"合同"（对应设计文档第七节、10.4 约束一）。
> **本版已与前端仓库 `Physical-Simulation-Platform-in-Wechat-main` 的实际调用逐一对齐。**
> **v0.3：全部 12 个接口已在 `router.go`+`main.go` 装配接线，可端到端联调。**
> **v0.4（2026-08-10）：对照任务书补齐 5 个接口——§14 班级详情 / §15 班级改名 / §16 移除成员（M2 系列 RESTful 补全）、§17 实验列表（M3）、§18 学生查自己审计日志（M6 的 `/audit/mine`）。已全量接线+真库实测。**
> **v0.5（2026-08-12）：① §12 提交接口新增可选 `user_config` 字段，解决"滑块改仿真真值但评分 target 固定"问题；② 审计日志补全--登录、班级 CRUD、成员增删均写 `operation_logs`（M6 验收要求"登录/建班有日志"）。**
> **v0.6（2026-08-25）：数值分析实验批次启动（后端+数据组）。L1 二分法求根（experiment/level id 11）上线；评分框架改注册表模式；§12 `user_config` 表新增 bisection 行；新增附录“数值分析实验批次”。**
> **v0.7（2026-08-25）：按《开发任务讲解》恢复 PDF 交付实验--牛顿环（level 1）+ 示波器（level 2），readings 与前端页面提交逐一对齐；打分曲线参数化为 target.bands（缺省回退数值分析曲线）；解锁链重排为 物理实验(1-2) -> 数值分析(11-12)；新增附录“物理实验批次”。**
> **v0.8（2026-08-26）：数值分析 L3 高斯消去法解线性方程组（experiment/level id 13，插队上线）--target.method=`linear_rel_err`，后端列主元高斯现算真值解，无穷范数相对误差打分；§12 `user_config` 表新增 gauss_elim 行（可换库内方程组或自定义 A/b）；解锁链延长为 ... -> L12 牛顿迭代 -> L13 高斯消去。**
> **v0.9（2026-08-27）：数值分析 L4 雅可比迭代法解线性方程组（experiment/level id 14，插队上线）--target.method=`jacobi_rel_err`，真值解=高斯直接法精确解，期望迭代数=后端从学生 x0 模拟雅可比；迭代效率罚分防直接法冒充（方向与牛顿实验相反）；§12 `user_config` 表新增 jacobi_iter 行；解锁链延长为 ... -> L13 高斯消去 -> L14 雅可比迭代。**
> **v0.10（2026-09-09）：删除物理实验批次--牛顿环（level 1）与示波器（level 2）的全部数据（migration `011_remove_ring_oscilloscope.sql`：experiments/levels/user_progresses/operation_logs 对应行删除，评分器与前端页面注册同步移除）；解锁链改为 L11 二分法(order 1) -> L12 牛顿迭代(order 2) -> L13 高斯消去(order 3) -> L14 雅可比迭代(order 4)，L11 变为链头（无前置）；§12 `user_config` 表与各接口示例中的物理实验引用全部清理，"物理实验批次"附录下线（历史格式见 v0.7-v0.9 版本）。**
> **v0.11（2026-09-10）：数值分析 L5 SOR 逐次超松弛迭代法解线性方程组（experiment/level id 15，插队上线）--target.method=`sor_rel_err`，真值解=高斯直接法精确解，期望迭代数=后端从学生 x0 与 ω 模拟 SOR（ω 以学生实际提交为准）；新增方程组 `sys6_poisson`（一维泊松差分三对角，SOR 主场）；§12 `user_config` 表新增 sor_iter 行（可换方程组/初值/ω）；解锁链延长为 ... -> L14 雅可比迭代(order 4) -> L15 SOR(order 5)。**
> **v0.12（2026-09-12）：数值分析 L6 拉格朗日插值（experiment/level id 16）上线--target.method=`interp_rel_err`，真值=插值多项式 P(x_eval)（由节点唯一确定，后端现算）；新增插值题库（sin_quad / sqrt_table / runge_5，runge_5 为龙格现象题，抄原函数值与插值真值差 ~1.15 倍相对误差天然防"不插值抄 f 值"）；§12 `user_config` 表新增 lagrange_interp 行（可换题/换求值点/自定义节点）；解锁链延长为 ... -> L15 SOR(order 5) -> L16 拉格朗日插值(order 6)。2026-09-16 真库 e2e 全通过。**
> **v0.12.1（2026-09-16）：L6 求值点归属修正--`target.x_eval` 仅属于 target 自己的题：user_config 换 `problem_key`/自定义节点却不带 `x_eval` 时，改用新题的库内默认求值点（此前沿用 target 的，e2e 中发现换 runge_5 后真值错按 sin_quad 求值点计算）；附回归测试 `TestScoreLagrangeInterp_TargetXEvalOnlyForTargetProblem`。**
> **v0.13（2026-09-16）：数值分析 L7 复合辛普森积分法（experiment/level id 17）上线--target.method=`integ_rel_err`，真值=同一 f、区间、n 下后端现算的复合辛普森值 S_n（"真值=方法输出"，同 L16 真值=P 的设计；精确积分 I 仅作审计参考记入 detail，抄 I 被截断误差压住：sin_pi 默认 n=2 抄 I=2 约 42 分不过线）；新增积分题库（sin_pi / erf_gauss / sqrt_x）；§12 `user_config` 表新增 simpson_integration 行（可换题/换区间/换 n）；target 的题专属参数（a/b/n）不跨题沿用（v0.12.1 求值点归属规则同款）；解锁链延长为 ... -> L16 拉格朗日插值(order 6) -> L17 复合辛普森积分法(order 7)。2026-09-17 真库 e2e 全通过。**
> **v0.14（2026-09-17）：数值分析 L8 逐次线性插值法 Aitken/Neville（experiment/level id 18，用户点名插队）上线--target.method=`aitken_rel_err`，真值=Neville 递推终值 P_{n−1,n−1}（与 L6 拉格朗日公式评同一插值多项式，两路计算数学上相等、单测交叉验证；题库与 L6 共用 interpProblems，龙格题抄 f 值同样 0 分）；本关与 L6 的区别在**过程**：每一步都是相邻低阶结果的线性插值逐列推进；readings 增 `col1` 审计字段（学生算的第 1 列线性插值结果）；§12 `user_config` 表新增 aitken_interp 行（可换题/换求值点/自定义节点，语义同 L6）；求值点归属规则同 v0.12.1；解锁链延长为 ... -> L17 复合辛普森积分法(order 7) -> L18 逐次线性插值法(order 8)。**
> **v0.15（2026-09-17）：数值分析 L9 龙贝格积分法 Romberg（experiment/level id 19，用户点名插队）上线--target.method=`romberg_rel_err`，真值=龙贝格表终值 T_k^(k)（"真值=方法输出"；后端按同一 f、区间、k 现算）；题库与 L7 共用 integProblems（sin_pi / erf_gauss / sqrt_x），外推第 1 列恰为复合辛普森 S_{2^m}（单测 `TestScoreRombergIntegration_FirstColumnEqualsSimpson` 结构性交叉验证，sin_pi 默认 k=2 时 S_1=2π/3 正是 L7 的 S_2）；readings 增 `t_col` 审计字段（学生算的梯形列 T_0..T_k，长度 k+1）；§12 `user_config` 表新增 romberg_integration 行（可换题/换区间/换外推深度 k=1~10）；题专属参数（a/b/k）归属规则同 v0.12.1；防作弊语义：抄精确积分 I=2 约 81 分（能过线但非满分，龙贝格收敛快），抄 L7 答案 2π/3 或最后一行梯形 T_2 均 ~41 分不过线；解锁链延长为 ... -> L18 逐次线性插值法(order 8) -> L19 龙贝格积分法(order 9)。**
> **v0.16（2026-09-21）：数值分析 L10/L11 欧拉法解常微分方程初值问题（experiment/level id 20/21，用户点名双关一次上线：L20 显式欧拉 + L21 改进欧拉）--target.method=`euler_rel_err` / `euler_improved_rel_err`，真值=方法输出 y_N（后端按同一 f、初值 y0、区间、步长 n 现算，"真值=方法输出"同 L7/L9 设计；精确解闭式记审计 ref_Y 不打分）；新增常微分方程题库 odeProblems（exp_growth / decay_sq / stiff_decay，均 [0,1] 默认 n=4，stiff_decay 为刚性绝对稳定性观察题：hλ=-2.5 时两法均振荡发散、真值仍确定，抄精确解直接 0 分）；readings 增 `y_col` 审计字段（学生算的逐点列 y_0..y_N，长度 n+1）；§12 `user_config` 表新增 euler_explicit / euler_improved 行（可换题/换区间/换步数 n=1~1024）；题专属参数（a/b/n）归属规则同 v0.12.1；防作弊语义：L20 抄精确解 e ~28 分、L21 抄精确解 ~49 分 / 抄 L20 显式答案 ~34 分均不过线；解锁链延长为 ... -> L19 龙贝格积分(order 9) -> L20 显式欧拉(order 10) -> L21 改进欧拉(order 11)。**
> **v0.17（2026-09-22）：数值分析 L12 四阶龙格-库塔法（experiment/level id 22，原排队表末位按序上线）--target.method=`rk4_rel_err`，真值=方法输出 y_N（后端用 rk4Column 现算，K1..K4 四次斜率探测加权平均，全局截断误差 O(h⁴)，与 L20/L21 构成精度阶对照 O(h)/O(h²)/O(h⁴)；题库复用 odeProblems，target 默认 exp_growth n=1 单步四阶：K1..K4=1、1.5、1.75、2.75 全手算，终值恰 65/24≈2.708333）；readings 格式与 L20/L21 完全同款（`{value, y_col?}`）；§12 `user_config` 表新增 runge_kutta_ode 行（可换题/换区间/换步数 n=1~1024）；题专属参数（a/b/n）归属规则同 v0.12.1；防作弊语义：抄精确解 e 约 72 分（RK4 收敛快，能过线但非满分--过程真伪由 y_col 审计暴露，L19 同款语义），抄 L21 改进欧拉答案 ~51 分、抄 L20 显式答案 ~30 分均不过线，stiff_decay 单步即发散（|R(−10)|=291）抄精确解与抄低阶方法发散值均 0 分；解锁链延长为 ... -> L21 改进欧拉(order 11) -> L22 龙格-库塔(order 12)；排队表移除 runge_kutta_ode（仅剩 least_squares_fit）。**
> **v0.18（2026-10-01）：数值分析 L13 阿当姆斯预报-校正法（experiment/level id 23，用户点名"微分方程数值解"方向）--target.method=`adams_rel_err`，真值=方法输出 y_N（后端用 adamsPCColumn 现算：线性多步法 AB4 外推预报 + AM4 内插校正一次【PECE 模式】，表头 y_1..y_3 用 RK4 启动【与 L22 同式，表头同阶精度】，全局截断误差 O(h⁴)，与 L20/L21/L22 单步法构成方法族对照）；题库复用 odeProblems，target 默认 exp_growth n=4（3 步 RK4 表头 + 1 步 AB4-AM4：表头 y_1..y_3 = R(1/4)^i，终值 2.7182259833）；readings 格式与 L20-L22 完全同款（`{value, y_col?}`）；§12 `user_config` 表新增 adams_pc_ode 行（可换题/换区间/换步数 **n=4~1024**——3 步表头+至少 1 步阿当姆斯，n<4 直接 400）；题专属参数（a/b/n）归属规则同 v0.12.1；防作弊语义：四阶法收敛快，抄精确解 e 约 93 分、抄 L22 默认答案 65/24 约 72 分均过线但非满分（同为四阶法数学上难分--过程真伪由 y_col 审计暴露，L19/L22 同款语义），抄 L21 改进欧拉答案 ~49 分、抄 L20 显式答案 ~30 分均不过线；stiff_decay（hλ=-2.5）多步法绝对稳定区间远小于 RK4，抄精确解 e^{-10} 直接 0 分、抄 RK4 n=4 表头答案 ~9 分也被拦；解锁链延长为 ... -> L22 龙格-库塔(order 12) -> L23 阿当姆斯预报-校正(order 13)。**
> **v0.19（2026-10-01）：数值分析 L14 最小二乘拟合（experiment/level id 24，原排队表末位按序上线，**数值分析链收官**）--target.method=`lsq_rel_err`，真值=法方程拟合多项式 P(x_eval)（后端 lsqFit 现算：幂基法方程 Σ_k x_k^{i+j}·c_i = Σ_k x_k^j·y_k【j=0..m】，列主元高斯求解，"真值=方法输出"同 L7/L9/L16/L20-L23 设计）；新增拟合题库 fitProblems（lin_line 直线拟合 4 点【法方程手算 a=1.1、b=0.96，P(3.5)=4.46】/ quad_parabola 二次拟合 5 点【系数恰 27/350、1/28、137/140，P(2)=711/175】/ exp_lnw 指数模型 y=a·e^{bx} 5 点【ln 线性化拟合，真值=e^{A+2.5B}=6.9103931】）；readings 增 `coef` 审计字段（学生解法方程得的系数 [c_0..c_m]，长度须 m+1）；§12 `user_config` 表新增 least_squares_fit 行（可换题/换拟合次数 m=0~min(5, 点数-2)【上界防拟合退化成插值】/换求值点，或自定义数据点 pts_x+pts_y 3~12 个且 x 互异、须同时给 x_eval）；题专属参数（m/x_eval）归属规则同 v0.12.1；防作弊语义：拟合不过点，抄观测值约 29/30/44 分、抄均值 16 分、指数题不线性化直接拟 y 12 分均不过线，quad 题 x_eval 落在数据点上抄真函数 x²=4 恰 58 分差线 2 分；抄三次插值 P3(3.5)=4.4375 约 69 分过线但非满分（插值≠拟合，与 RK4 抄 e 同款语义，过程真伪由 coef 审计暴露）；解锁链延长为 ... -> L23 阿当姆斯预报-校正(order 13) -> L24 最小二乘拟合(order 14)；排队表清空。**
> **v0.20（2026-10-03）：数值分析 L15 三次样条插值（experiment/level id 25，用户点名方向插队上线，插值系列补最后经典，难度 4）--target.method=`spline_rel_err`，真值=自然边界三次样条 S(x_eval)（后端 splineMoments+splineEvalAt 现算：三弯矩方程 mu_i·M_{i-1}+2M_i+lam_i·M_{i+1}=d_i【自然边界 M_0=M_{n-1}=0】三对角**追赶法**求解，"真值=方法输出"同 L7/L9/L16/L20-L24 设计）；新增样条题库 splineProblems（dist_4 位移观测数据 4 节点不等距 h=1,1,2【三弯矩手算闭式 M1=15/23、M2=78/23，真值 S(3)=559/46】/ sin_5 sin 五等距节点【对称 M1=M3，真值 S(π/6)≈0.49974——光滑函数上样条逼近极好，抄 sin(π/6)=0.5 约 83 分过线但非满分，过程真伪由 m_col 审计暴露，L19 抄 I / L22 抄 e 同款语义】/ runge_5 龙格函数五等距节点【与 L16 拉格朗日同节点同求值点三法对照：P(0.8)=−0.379310 剧烈振荡 vs 样条 S(0.8)≈−0.036340，抄 P/抄 f(0.8)=1/17/抄观测全 0 分，样条治龙格的主场题；教学点：自然边界端段仍下弯，clamped 边界更准但需导数信息】）；readings 增 `m_col` 审计字段（学生解出的弯矩 M_0..M_{n-1}，长度须=节点数、含端点零弯矩）；§12 `user_config` 表新增 cubic_spline 行（可换题/换求值点，或自定义节点 nodes_x+nodes_y 3~8 个且**x 严格递增**、须带 x_eval 且**在节点区间内——样条不做外推**，与 L16 可外推的差异点）；题专属参数（x_eval）归属规则同 v0.12.1；防作弊语义：dist_4 抄拉格朗日 P(3)=11.5 约 39 分、抄线性内插 13 约 35 分、抄观测全 0 分均不过线，只有真解三弯矩才过线；自定义节点抄原函数（如 x² 节点抄 6.25）约 58 分恰差线 2 分（自然边界端段偏差）；解锁链延长为 ... -> L24 最小二乘拟合(order 14) -> L25 三次样条插值(order 15)。**
> **v0.21（2026-10-09）：数值分析 L16 隐式（后退）欧拉法（experiment/level id 26，用户点名"微分方程数值解"再添一关，难度 3）--target.method=`euler_implicit_rel_err`，真值=方法输出 y_N（后端 eulerImplicit 现算：y_{n+1}=y_n+h·f(x_{n+1},y_{n+1}) 隐式方程，题库 odeProblems 均为线性齐次 y'=a(x)·y 故闭式解 **y_{n+1}=y_n/(1−h·a(x_{n+1}))** 无须迭代，"真值=方法输出"同 L20-L23 设计；稳定性函数 R(hλ)=1/(1−hλ)，绝对稳定域 |1−z|≥1 含整个左半平面——**A 稳定**，与 L20 显式欧拉圆盘 |1+z|≤1 互补，stiff_decay（hλ=−2.5）上显式振荡发散 (−1.5)^4=5.0625 vs 隐式单调收敛 (2/7)^4=16/2401，显隐正面对照的绝对稳定性教学主场）；题库复用 odeProblems，target 默认 exp_growth n=4：每步除以 (1−h) 恰乘 4/3，真值闭式 **(4/3)^4=256/81≈3.160494**（隐式比显式 2.44 离精确解 e 更远——单向低估，从上方压向真解）；readings 格式与 L20-L23 完全同款（`{value, y_col?}`）；§12 `user_config` 表新增 euler_implicit 行（可换题/换区间/换步数 n=1~1024）；题专属参数（a/b/n）归属规则同 v0.12.1；防作弊语义：抄精确解 e 约 24 分、抄 L20 显式答案 2.44140625 约 15 分、抄 L21 改进欧拉约 21 分、抄 L22 RK4 约 23 分、抄 L23 阿当姆斯约 24 分均不过线（一阶法截断误差 ~14% 全拦截）；decay_sq 抄精确解 e^{−1} 约 36 分不过线；stiff_decay 抄精确解 e^{−10} 与抄 L20 显式发散值 5.0625 均 0 分；**边界：h=1（exp_growth n=1）时 1−h·f(x,1)=0 隐式方程无有限解 → 400**；解锁链延长为 ... -> L25 三次样条插值(order 15) -> L26 隐式欧拉法(order 16)。**
> **v0.22（2026-10-09）：数值分析 L17 LU 分解（Doolittle）解线性方程组（experiment/level id 27，用户点名"继续加实验"，AskUserQuestion 选定方向，难度 3）--target.method=`lu_rel_err`，真值=LU 分解路径解（后端 luDecompose 现算：**Doolittle 不选主元** A=LU——消去乘数 m_ik 存入 L（单位下三角）、消去结果即 U（上三角），再前代 Ly=b + 回代 Ux=y，"真值=方法输出"同全系列设计；与 L13 列主元高斯消去同族：消去过程数学相同，LU 把乘数存下来可复用解多组 b）；题库复用 linSystems（L13 同款五题均无需选主元即可分解），target 默认 sys3_basic：解恰 (2,3,-1)，乘子列手算闭式 **[-1.5,-1,4]**、U 对角 2/0.5/-1；readings：`{solution 必填（长度=未知数个数）, l_col? 可选审计}`——l_col=L 的乘子按行序 l21,l31,l32,...，长度须 **n(n-1)/2**，仅记 detail 不参与打分（过程真伪由乘子审计暴露）；user_config 与 L13 完全同款（换 system_key 或自定义 A/b 2~8 阶）；防作弊语义：解与 L13 数学同源（同 L18/L16 先例：**抄=算对**），抄 b 向量/零向量/分量换位低分不过线；**边界：自定义矩阵主元≈0（如 [[0,1],[1,0]] 非奇异但首主元为 0）或奇异 → 400**——不选主元的 LU 会中断，正是与 L13 列主元稳定性的差异教学点；解锁链延长为 ... -> L26 隐式欧拉法(order 16) -> L27 LU 分解解线性方程组(order 17)。**
> **v0.23（2026-10-09）：数值分析 L18 追赶法解三对角方程组（experiment/level id 28，用户点名"做追赶法"，L27 LU 分解的三对角特例，难度 2）--target.method=`thomas_rel_err`，真值=追赶法解（后端 thomasChase 现算：**追**（Crout 消元）beta_1=b_1，m_i=a_{i-1}/beta_{i-1}，beta_i=b_i−m_i·c_{i-1}，d_i←d_i−m_i·d_{i-1}；**赶**（回代）x_n=d_n/beta_n，x_i=(d_i−c_i·x_{i+1})/beta_i，仅存三条对角线 O(n) 计算量，"真值=方法输出"同全系列设计；单测含题库四题+自定义阵 追赶 vs 列主元高斯 vs LU 三路恒等交叉验证）；新增三对角题库 triSystems（紧凑 a/b/c/d 存储，均对角占优或对称正定——追赶法不选主元必安全）：tri1_basic 手算例题 4 阶【m=(0.5,0.8,0.3125)，beta=(2,2.5,3.2,4.6875)，解恰 (1,2,3,4)】/ tri2_poisson 一维泊松差分 5 内点【beta=(2,3/2,4/3,5/4,6/5)，解全 1】/ tri3_small 入门 3 阶【解 (1,1,1)】/ tri4_spline 三弯矩系数阵 6 阶【L25 呼应，解全 1】；readings：`{solution 必填（长度=未知数个数）, m? 可选审计（乘子 m2..mn，长 n-1）, beta? 可选审计（新主对角 beta1..betan，长 n）}`——双审计各全或无、仅记 detail 不参与打分；§12 `user_config` 表新增 thomas_solve 行（换 system_key 或自定义三对角 a/b/c/d 3~8 阶）；防作弊语义：解与 L13/L27 数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 m/beta 双审计暴露；**边界：beta≈0（如 b1=0 非奇异但需选主元）→ 400**——追赶法不选主元会中断，适用条件（对角占优/对称正定）的教学点；解锁链延长为 ... -> L27 LU 分解解线性方程组(order 17) -> L28 追赶法解三对角方程组(order 18)。**
> **v0.24（2026-10-11）：数值分析 L19 平方根法（Cholesky）解对称正定方程组（experiment/level id 29，用户点名"继续补充新的实验"，直接法一章收官，难度 2）--target.method=`cholesky_rel_err`，真值=平方根法解（后端 choleskySolve 现算：**A = L·Lᵀ** 按行 Cholesky–Crout 分解——l_ii=sqrt(a_ii−Σ_{k<i} l_ik²)，l_ij=(a_ij−Σ_{k<j} l_ik·l_jk)/l_jj（j<i，L 主对角全正），再前代 Ly=b + 回代 Lᵀx=y，"真值=方法输出"同全系列设计；单测含题库四题+自定义阵 **LLᵀ 重建恒等式** 与 平方根法 vs 列主元高斯 vs LU 三路同解交叉验证）；新增对称正定题库 spdSystems（L/y/x 全整数，课堂手算友好）：spd1_2x2 入门 2 阶【L=[[2,0],[1,2]]，y=(3,2)，解恰 (1,1)】/ spd2_min3 min(i,j) 阵 3 阶（默认题）【L=下三角全 1，y=(6,5,3)，解恰 (1,2,3)】/ spd3_tridiag3 三对角 SPD【L28 呼应，y=(3,3,3)，解恰 (1,1,1)】/ spd4_min4 min(i,j) 阵 4 阶【y=(4,3,2,1)，解全 1】；readings：`{solution 必填（长度=未知数个数）, L? 可选审计（n 阶方阵，对照取下三角含主对角 n(n+1)/2 个元）, y? 可选审计（前代中间向量，长 n）}`——双审计各全或无、仅记 detail 不参与打分；§12 `user_config` 表新增 cholesky_solve 行（换 system_key 或自定义 A/b 2~5 阶对称方阵）；防作弊语义：解与 L13/L27/L28 数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 L/y 双审计暴露；**边界：非对称 → 400，根号内≤0（不定阵 [[1,2],[2,1]] 或半正定奇异阵 [[1,1],[1,1]]）→ 400**——平方根法只适用于对称正定阵，换来约一半计算量且永不选主元，正是与 L13 列主元高斯的差异教学点；解锁链延长为 ... -> L28 追赶法解三对角方程组(order 18) -> L29 平方根法解对称正定方程组(order 19)。**
> 定稿后改契约必须全组知晓。

## 0. 通用约定

- Base URL（开发期）：`http://<后端电脑IP>:8080`（真机调试不能用 localhost）
- 除 `/auth/login`、`/ping` 外，所有接口需要请求头：`Authorization: Bearer <token>`
- Content-Type：`application/json`
- **成功响应直接返回数据本体**（不包 `{code:0,...}`）；前端 `request.js` 按 HTTP 状态码判成败
- **统一错误格式**（HTTP 状态码与 body.code 一致）：

```json
{ "code": 401, "message": "token expired" }
```

| code | 含义 | 前端动作 |
|------|------|---------|
| 400 | 参数错误 | toast message |
| 401 | 未登录 / token 失效 | request.js 自动清登录态、跳登录页 |
| 403 | 无权限（角色不符） | toast message |
| 404 | 资源不存在 | toast message |
| 429 | 触发限流 | toast 稍后再试 |
| 500 | 服务器错误 | toast message |

## 0.1 两个不一致点的处理决定（2026-07-30 定）

1. **登录路径**：规范路径为 `POST /auth/login`。前端 `utils/auth.js` **已修正为 `/auth/login`**（与 `app.js` 一致）。后端保留别名 `POST /login` 一个迭代作灰度，确认无误后删除。
2. **实验配置参数**：牛顿环页调 `GET /experiments/:id` 时传的是 **level_id**（不是 experiment_id）。决定：**后端该接口按 level_id 解析**（level → experiment → config），响应里的 `id` 返回 level_id。契约以此为准。

---

## 1. POST /auth/login（别名 POST /login）

无需鉴权。前端 `wx.login()` 拿临时 code 后调用。

**请求**

```json
{ "code": "0a3xxx..." }
```

**响应 200**

```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": 1,
    "openid": "oABCD...",
    "role": "student",
    "name": null,
    "need_complete": true
  }
}
```

- `role`：`student` / `teacher` / `admin`
- `name` 为 `null` 且 `need_complete=true` → 前端切到"补全学籍信息"模式
- **开发期后门**：`code` 以 `dev_` 开头时（如 `dev_student`），不请求微信接口，直接使用内置测试账号签发 token。上线前必须移除。

**错误**：400（code 为空或微信侧校验失败）

## 2. GET /auth/me

鉴权：JWT。返回当前登录用户，结构同 #1 的 `user`。

## 3. PUT /users/:id

鉴权：JWT（本人）。补全学籍信息。

**请求**

```json
{ "name": "李同学", "student_no": "2023001", "class_id": 1 }
```

**响应 200**：更新后的 user 对象（前端用 `{...user, ...res.data}` 合并，`need_complete` 由前端置 false）

**错误**：400（name/student_no 为空）、403（改别人的信息）、404

## 4. GET /classes

鉴权：JWT。
- 学生：返回自己加入的班级
- 教师/管理员：返回自己带的班级（admin 全部）

**响应 200**

```json
[
  { "id": 1, "name": "物理实验1班", "teacher_name": "张老师", "member_count": 32 },
  { "id": 2, "name": "物理实验2班", "teacher_name": "李老师", "member_count": 28 }
]
```

## 5. POST /classes

鉴权：JWT，角色 `teacher`/`admin`。

**请求**：`{ "name": "物理实验3班" }`
**响应 200**：创建的班级对象（含 id）

## 6. DELETE /classes/:id

鉴权：JWT，本班教师/admin。**响应 200**：`{ "message": "deleted" }`

## 7. POST /classes/:id/members

鉴权：JWT，本班教师/admin。

**请求**：`{ "user_id": 12 }`
**响应 200**：`{ "message": "added" }`；重复添加返回 409 `{ "code": 409, "message": "already a member" }`

## 8. GET /levels

鉴权：JWT。**按当前用户个性化**：每关带 `status` 和 `best_score`。

**响应 200**

```json
[
  { "id": 11, "name": "二分法求根", "experiment_code": "bisection",
    "status": "passed", "difficulty": 1, "best_score": 100 },
  { "id": 12, "name": "牛顿迭代法求根", "experiment_code": "newton_iter",
    "status": "unlocked", "difficulty": 2, "best_score": 0 },
  { "id": 13, "name": "高斯消去法解线性方程组", "experiment_code": "gauss_elim",
    "status": "locked", "difficulty": 2, "best_score": 0 },
  { "id": 14, "name": "雅可比迭代法解线性方程组", "experiment_code": "jacobi_iter",
    "status": "locked", "difficulty": 2, "best_score": 0 },
  { "id": 15, "name": "SOR 逐次超松弛迭代法解线性方程组", "experiment_code": "sor_iter",
    "status": "locked", "difficulty": 2, "best_score": 0 },
  { "id": 16, "name": "拉格朗日插值", "experiment_code": "lagrange_interp",
    "status": "locked", "difficulty": 2, "best_score": 0 },
  { "id": 17, "name": "复合辛普森积分法", "experiment_code": "simpson_integration",
    "status": "locked", "difficulty": 3, "best_score": 0 },
  { "id": 18, "name": "逐次线性插值法", "experiment_code": "aitken_interp",
    "status": "locked", "difficulty": 2, "best_score": 0 },
  { "id": 19, "name": "龙贝格积分法", "experiment_code": "romberg_integration",
    "status": "locked", "difficulty": 3, "best_score": 0 }
]
```

- `status`：`locked` / `unlocked` / `in_progress` / `passed`
- 解锁规则：第 1 关默认 unlocked；其余关前置关 passed 后解锁

## 9. GET /experiments/:id

鉴权：JWT。**:id 是 level_id**（见 0.1-2）。

**响应 200**

```json
{
  "id": 11,
  "code": "bisection",
  "name": "二分法求根",
  "render_mode": "canvas_2d",
  "config": {
    "func": "x^3 - x - 1 = 0",
    "func_key": "cubic_1",
    "interval": [1.0, 1.5],
    "tolerance": 1e-6,
    "max_iterations": 50,
    "expected_iterations": 19
  },
  "target": { "method": "root_rel_err" }
}
```

- `config` 结构随实验类型不同（MySQL JSON 字段）

## 10. GET /progress/mine

鉴权：JWT（学生看自己）。

**响应 200**

```json
{
  "total": 4,
  "passed": 1,
  "avg_score": 100,
  "best_score": 100,
  "records": [
    { "id": 11, "level_name": "二分法求根", "status": "passed", "score": 100, "best_score": 100, "attempts": 3 },
    { "id": 12, "level_name": "牛顿迭代法求根", "status": "in_progress", "score": 0, "best_score": 0, "attempts": 1 }
  ]
}
```

## 11. GET /progress/class/:classId

鉴权：JWT，本班教师/admin。

**响应 200**

```json
{
  "class": { "id": 3, "name": "物理一班", "teacher": "张老师" },
  "summary": { "avg_score": 82.4, "pass_rate": 0.91 },
  "rows": [
    { "user_id": 12, "name": "李同学", "student_no": "2023001",
      "best_score": 90, "attempts": 3, "passed": true }
  ]
}
```

## 12. POST /progress/submit

鉴权：JWT，角色 `student`。

**请求**

```json
{
  "level_id": 11,
  "experiment": "bisection",
  "readings": [ { "root": 1.324718, "iterations": 19 } ],
  "user_config": { "func_key": "cos_x_x", "interval": [0.0, 1.0] }
}
```

**`user_config`（v0.5 新增，可选）**

前端实验页可能改变仿真参数（v0.5 为物理滑块引入；现为数值实验的换函数/换方程组/换初值机制）。
若提交时携带 `user_config`，后端用它覆盖 DB target 中对应的参数后重算真值再评分，确保"学生换题/调参后认真完成仍能得高分"。
不携带时回退到 DB 固定 target（向后兼容）。

| 实验 | user_config 可覆盖字段 |
|------|----------------------|
| bisection | `func_key`, `interval`（换题/换区间，后端重算真值根） |
| newton_iter | `func_key`, `x0`（换题/换初值，后端模拟牛顿重算真值与期望迭代数） |
| gauss_elim | `system_key`（换库内方程组）或 `A`/`b`（自定义方程组，2~8 阶方阵；后端重算真值解） |
| jacobi_iter | `system_key`（换库内方程组）或 `A`/`b`（自定义，2~8 阶方阵且对角元非零）；`x0`（迭代初值，后端按同初值模拟重算期望迭代数） |
| sor_iter | `system_key`（换库内方程组）或 `A`/`b`（自定义，2~8 阶方阵且对角元非零）；`x0`（迭代初值）；`omega`（松弛因子 (0,2)，后端按学生实际 ω 模拟重算期望迭代数） |
| lagrange_interp | `problem_key`（换库内插值题）或 `nodes_x`/`nodes_y`（自定义节点，2~8 个且 x 互异，须同时给 `x_eval`）；`x_eval`（换求值点，后端按新节点/新求值点重算插值真值） |
| simpson_integration | `problem_key`（换库内积分题）；`a`+`b`（换积分区间，须成对给出且 a<b）；`n`（偶数子区间数 2~1024，后端按同 f、区间、n 重算辛普森真值 S_n） |
| aitken_interp | `problem_key`（换库内插值题，题库与 L6 共用）或 `nodes_x`/`nodes_y`（自定义节点，2~8 个且 x 互异，须同时给 `x_eval`）；`x_eval`（换求值点，后端按新节点/新求值点用 Neville 递推重算真值） |
| romberg_integration | `problem_key`（换库内积分题，题库与 L7 共用）；`a`+`b`（换积分区间，须成对给出且 a<b）；`k`（外推深度 1~10，后端按同 f、区间、k 重算龙贝格真值 T_k^(k)） |
| euler_explicit | `problem_key`（换库内 ODE 题）；`a`+`b`（换区间，须成对给出且 a<b）；`n`（步数 1~1024，后端按同 f、初值 y0、区间、n 重算显式欧拉真值 y_N） |
| euler_improved | `problem_key`（换库内 ODE 题，题库与 L20 共用）；`a`+`b`（换区间，须成对给出且 a<b）；`n`（步数 1~1024，后端按同 f、初值 y0、区间、n 重算改进欧拉真值 y_N） |
| euler_implicit | `problem_key`（换库内 ODE 题，题库与 L20/L21 共用）；`a`+`b`（换区间，须成对给出且 a<b）；`n`（步数 1~1024，后端按同 f、初值 y0、区间、n 重算隐式欧拉真值 y_N——题库线性齐次故闭式 y_{n+1}=y_n/(1−h·a(x_{n+1}))；**h·a(x)=1 时隐式方程无有限解 → 400**） |
| runge_kutta_ode | `problem_key`（换库内 ODE 题，题库与 L20/L21 共用）；`a`+`b`（换区间，须成对给出且 a<b）；`n`（步数 1~1024，后端按同 f、初值 y0、区间、n 重算 RK4 真值 y_N） |
| adams_pc_ode | `problem_key`（换库内 ODE 题，题库与 L20-L22 共用）；`a`+`b`（换区间，须成对给出且 a<b）；`n`（步数 4~1024——3 步 RK4 表头+至少 1 步阿当姆斯，后端按同 f、初值 y0、区间、n 重算 AB4-AM4 真值 y_N） |
| least_squares_fit | `problem_key`（换库内拟合题）或 `pts_x`/`pts_y`（自定义数据点，3~12 个且 x 互异，须同时给 `x_eval`，默认 m=1）；`m`（拟合次数 0~min(5, 点数-2)——上界防拟合退化成插值）；`x_eval`（换求值点，后端按新数据点/新 m/新求值点重算法方程拟合真值 P(x_eval)） |
| cubic_spline | `problem_key`（换库内样条题）或 `nodes_x`/`nodes_y`（自定义节点，3~8 个且 **x 严格递增**，须同时给 `x_eval` 且在节点区间内——**样条不做外推**）；`x_eval`（换求值点，后端按新节点/新求值点重解三弯矩方程重算样条真值 S(x_eval)） |
| lu_decomp | `system_key`（换库内方程组，题库与 L13 共用）或 `A`/`b`（自定义方程组，2~8 阶方阵；后端重做 Doolittle 分解重算真值解——**主元≈0 或奇异 → 400**，不选主元的 LU 会中断） |
| thomas_solve | `system_key`（换库内三对角方程组，题库 triSystems 四题）或 `a`/`b`/`c`/`d`（自定义三对角：下对角/主对角/上对角/右端，长度 n-1/n/n-1/n，3~8 阶；后端重做追赶重算真值解——**beta≈0 → 400**，追赶法不选主元会中断，对角占优/对称正定阵必安全） |
| cholesky_solve | `system_key`（换库内对称正定方程组，题库 spdSystems 四题）或 `A`/`b`（自定义方程组：对称方阵/右端，2~5 阶；后端重做 LLᵀ 分解重算真值解——**非对称或根号内≤0（非正定）→ 400**，平方根法只适用于对称正定阵） |

**响应 200**

```json
{ "score": 100, "passed": true, "best_score": 100, "unlocked_level_id": 12 }
```

- 后端按实验公式算分；`best_score` 取历史最佳（防重复刷分）
- 每次提交写 `operation_logs` 审计表
- **限流**：每用户每分钟 20 次（`config.rate_limit.submit_per_minute`），超限 429

**错误**：400（readings 非法，含空数组）、403（非学生 / **关卡未解锁--前置未过关，v0.7 起服务端强制**）、429（刷分限流）

---

## 13. GET /admin/operation-logs

鉴权：JWT，角色 `admin`。分页查询操作审计日志（`operation_logs` 表只写不读的补全）。

**Query 参数**（均可选）：

| 参数 | 说明 |
|------|------|
| `user_id` | 按用户过滤 |
| `action` | 按动作过滤（如 `submit`） |
| `level_id` | 按关卡过滤 |
| `page` | 页码，默认 1 |
| `size` | 每页条数，默认 50，上限 200 |

**响应 200**

```json
{
  "page": 1, "size": 50, "total": 9,
  "records": [
    { "id": 9, "user_id": 2, "action": "submit", "level_id": 11, "score": 100, "detail": "root=1.324717957 rel_err=1e-15 true_root=1.324717957", "created_at": "2026-08-01T..." }
  ]
}
```

**错误**：403（非 admin）、401（未登录）

---

## 14. GET /classes/:id（v0.4 新增，M2 补全）

鉴权：JWT。**本班教师 / admin / 本班学生成员**可看；其余 403。

**响应 200**

```json
{
  "id": 1, "name": "物理实验1班", "teacher_id": 3, "teacher_name": "张老师",
  "members": [
    { "user_id": 2, "name": "测试同学", "student_no": "2023001", "joined_at": "2026-08-01T..." }
  ]
}
```

**错误**：404（班级不存在）、403（无权查看）

## 15. PUT /classes/:id（v0.4 新增，M2 补全）

鉴权：JWT，本班教师/admin。改班级名。

**请求**：`{ "name": "物理实验1班（秋）" }`
**响应 200**：更新后的班级对象
**错误**：400（name 为空）、403、404

## 16. DELETE /classes/:id/members/:userId（v0.4 新增，M2 补全）

鉴权：JWT，本班教师/admin。把成员移出班级。

**响应 200**：`{ "message": "removed" }`
**错误**：404（班级或成员关系不存在）、403

## 17. GET /experiments（v0.4 新增，M3 补全）

鉴权：JWT。实验元数据列表（不含 config/target，单项配置仍走 §9）。

**响应 200**

```json
[
  { "id": 11, "code": "bisection", "name": "二分法求根", "render_mode": "canvas_2d" },
  { "id": 12, "code": "newton_iter", "name": "牛顿迭代法求根", "render_mode": "canvas_2d" },
  { "id": 13, "code": "gauss_elim", "name": "高斯消去法解线性方程组", "render_mode": "canvas_2d" },
  { "id": 14, "code": "jacobi_iter", "name": "雅可比迭代法解线性方程组", "render_mode": "canvas_2d" }
]
```

## 18. GET /audit/mine（v0.4 新增，M6 的"学生看自己日志"）

鉴权：JWT（任意角色）。只看**当前用户自己**的审计日志，user_id 强制为本人，不接受过滤参数。

**Query 参数**：`page`（默认 1）、`size`（默认 50，上限 200）、`action`（可选过滤）

**响应 200**：结构同 §13（`{page,size,total,records}`）

---

## 附：跨域与限流（M2 中间件）

- **CORS**：`mode=debug` 放行所有 Origin；`mode=release` 收紧到 `config.cors.allow_origins` 白名单。OPTIONS 预检返回 204。小程序本身不受 CORS 约束，主要服务于 Apifox/本地 H5 调试。
- **限流**（内存令牌桶）：`/auth/login` 按 IP 每分钟 `login_per_minute`（默认 30）；`/progress/submit` 按 user_id 每分钟 `submit_per_minute`（默认 20）。超限返回 `429 {code:429,message:"请求过于频繁，请稍后再试"}` + `Retry-After` 头。

## 附：开发后门开关

`config.allow_dev_backdoor`（默认 `true`）：`true` 时 `code` 以 `dev_` 开头走内置账号、`POST /login` 别名可用；`false` 时 `dev_` code 被拒（400）、别名不注册。上线前设 `false` 并填好 `wechat.appid/secret`。

---

## 附：数值分析实验批次（v0.6 起，后端+数据组维护）

2026-08-25 起按"一步一个实验"节奏新增，每实验四件套：migration SQL（数据组）+ 评分函数 + 单测 + 本附录格式登记。**评分真值一律由后端按 target/user_config 参数现算，不落库手填**。

**编号约定（v0.10 起）**：数值分析实验 id 从 11 起（bisection=11、newton_iter=12、gauss_elim=13、jacobi_iter=14、sor_iter=15、lagrange_interp=16、simpson_integration=17、aitken_interp=18、romberg_integration=19、euler_explicit=20、euler_improved=21、runge_kutta_ode=22、adams_pc_ode=23、least_squares_fit=24、cubic_spline=25、euler_implicit=26、lu_decomp=27、thomas_solve=28、cholesky_solve=29）；物理实验 id 1-2（牛顿环/示波器）已随 v0.10 下线，1-10 空置保留。**解锁链（v0.24）**：L11 二分法(order 1，链头无前置) -> L12 牛顿迭代(order 2) -> L13 高斯消去(order 3) -> L14 雅可比迭代(order 4) -> L15 SOR(order 5) -> L16 拉格朗日插值(order 6) -> L17 复合辛普森积分法(order 7) -> L18 逐次线性插值法(order 8) -> L19 龙贝格积分法(order 9) -> L20 显式欧拉法(order 10) -> L21 改进欧拉法(order 11) -> L22 龙格-库塔法(order 12) -> L23 阿当姆斯预报-校正法(order 13) -> L24 最小二乘拟合(order 14) -> L25 三次样条插值(order 15) -> L26 隐式欧拉法(order 16) -> L27 LU 分解解线性方程组(order 17) -> L28 追赶法解三对角方程组(order 18) -> L29 平方根法解对称正定方程组(order 19)。

### 实验清单与进度

| level_id | exp_id | code | 实验名 | 难度 | 状态 |
|---|---|---|---|---|---|
| 11 | 11 | `bisection` | 二分法求根 | 1 | ✅ v0.6 上线 |
| 12 | 12 | `newton_iter` | 牛顿迭代法求根 | 2 | ✅ v0.6 上线 |
| 13 | 13 | `gauss_elim` | 高斯消去法解线性方程组 | 2 | ✅ v0.8 上线（插队） |
| 14 | 14 | `jacobi_iter` | 雅可比迭代法解线性方程组 | 2 | ✅ v0.9 上线（插队） |
| 15 | 15 | `sor_iter` | SOR 逐次超松弛迭代法解线性方程组 | 2 | ✅ v0.11 上线（插队） |
| 16 | 16 | `lagrange_interp` | 拉格朗日插值 | 2 | ✅ v0.12 上线 |
| 17 | 17 | `simpson_integration` | 复合辛普森积分法 | 3 | ✅ v0.13 上线 |
| 18 | 18 | `aitken_interp` | 逐次线性插值法 | 2 | ✅ v0.14 上线（插队） |
| 19 | 19 | `romberg_integration` | 龙贝格积分法 | 3 | ✅ v0.15 上线（插队） |
| 20 | 20 | `euler_explicit` | 显式欧拉法 | 2 | ✅ v0.16 上线（插队） |
| 21 | 21 | `euler_improved` | 改进欧拉法 | 3 | ✅ v0.16 上线（插队） |
| 22 | 22 | `runge_kutta_ode` | 龙格-库塔法解 ODE | 4 | ✅ v0.17 上线（插队） |
| 23 | 23 | `adams_pc_ode` | 阿当姆斯预报-校正法解 ODE | 4 | ✅ v0.18 上线（插队） |
| 24 | 24 | `least_squares_fit` | 最小二乘拟合 | 3 | ✅ v0.19 上线 |
| 25 | 25 | `cubic_spline` | 三次样条插值 | 4 | ✅ v0.20 上线（插队） |
| 26 | 26 | `euler_implicit` | 隐式欧拉法 | 3 | ✅ v0.21 上线（插队） |
| 27 | 27 | `lu_decomp` | LU 分解解线性方程组 | 3 | ✅ v0.22 上线（插队） |
| 28 | 28 | `thomas_solve` | 追赶法解三对角方程组 | 2 | ✅ v0.23 上线（插队） |
| 29 | 29 | `cholesky_solve` | 平方根法解对称正定方程组 | 2 | ✅ v0.24 上线（插队） |

### L1 bisection 二分法求根

**config（GET /experiments/:id 下发）**

```json
{ "func": "x^3 - x - 1 = 0", "func_key": "cubic_1", "interval": [1.0, 1.5],
  "tolerance": 1e-6, "max_iterations": 50, "expected_iterations": 19 }
```

**readings（POST /progress/submit）**

```json
[ { "root": 1.324718, "iterations": 19 } ]
```

- `root` 必填（缺字段/NaN/Inf -> 400）；`iterations` 可选，仅记入审计日志
- **target.method = `root_rel_err`**：后端按 `func_key` + `interval` 现算真值根，按 `|root-真值|/|真值|` 打分；`user_config` 可覆盖 `func_key`/`interval`（换题防背答案）
- 真值根示例：cubic_1 在 [1.0,1.5] 的根 ≈ 1.3247179572

**函数库（func_key，后端持真值，前端仅用 config 展示）**：`cubic_1`（x^3-x-1，唯一实根≈1.324718）、`cos_x_x`（cos x-x，唯一根≈0.739085）、`sqrt_2`（x^2-2，双根±√2）。新增函数改 `scoring.go` 的 `numFuncs` 并同步此处。

**打分曲线（数值分析批次默认，拐点间 log10 线性插值）**：relErr<=1e-6 -> 100；1e-4 -> 90；1e-3 -> 80；1e-2 -> 65；0.1 -> 30；>=0.5 -> 0。及格 pass_score=60（约 relErr <= 0.0139）。v0.7 起可用 `target.bands` 覆盖（`[[relErr,score],...]` 按读数容差量级标定的打分曲线，relErr 严格升序、拐点间对数插值，数据组改曲线只动 target JSON）。

### L2 newton_iter 牛顿迭代法求根（v0.6）

**config（GET /experiments/:id 下发）**

```json
{ "func": "x^3 - x - 1 = 0", "func_key": "cubic_1", "interval": [1.0, 1.5],
  "x0": 1.5, "tolerance": 1e-6, "max_iterations": 50, "expected_iterations": 4 }
```

与 L1 同函数（cubic_1），便于对比两法收敛速度：牛顿 4 步 vs 二分 19 步。

**readings（POST /progress/submit）**

```json
[ { "root": 1.324718, "iterations": 4, "x0": 1.5 } ]
```

- `root` 必填（缺字段/NaN/Inf -> 400）；`iterations` 须在 1~max_iterations（缺省不罚分）；`x0` 可选（缺省依次回退 `user_config.x0` / `target.x0`）
- **target.method = `newton_rel_err`**：后端按学生 `x0` 模拟牛顿迭代（停止准则 `|x_{n+1}-x_n| < tolerance`）得出**真值根**与**期望迭代数**；多根函数（如 sqrt_2）从负初值收敛到负根，按学生实际路径评分
- 模拟不收敛（f'=0 / 发散 / 超上限）时回退 target 区间二分真值，且不罚迭代分；换题后若兜底区间不夹根 -> 400
- **迭代效率罚分**：`4 x max(0, iterations - expected - 2)`（宽限 2 步覆盖停止准则差异）。例：根正确但报 19 次（二分冒充）-> 罚 52 分得 48，不及格
- 审计 detail 记录 `x0 / iterations / expected_iterations / penalty`，教师端可查

### L3 gauss_elim 高斯消去法解线性方程组（v0.8，2026-08-26 插队上线）

**config（GET /experiments/:id 下发）**

```json
{ "system_key": "sys3_basic",
  "system_desc": "2x + y - z = 8; -3x - y + 2z = -11; -2x + y + 2z = -3",
  "A": [[2,1,-1],[-3,-1,2],[-2,1,2]], "b": [8,-11,-3], "tolerance": 1e-6 }
```

**readings（POST /progress/submit）**

```json
[ { "solution": [2, 3, -1] } ]
```

- `solution` 必填，长度须等于未知数个数（不匹配/含 NaN/Inf -> 400）
- **target.method = `linear_rel_err`**：后端用**列主元高斯消去法**现算真值解 x*（真值不落库），按无穷范数相对误差 `||x-x*||inf / ||x*||inf` 打分；缺省走数值分析默认曲线，`target.bands` 可覆盖
- **user_config 两种换题方式**：`{"system_key":"..."}` 换库内方程组；`{"A":[[...]],"b":[...]}` 自定义方程组（A/b 优先于 system_key；2~8 阶方阵、元素须有限，奇异矩阵 -> 400）
- sys3_basic 解恰为整数 (2, 3, -1)，手工消去即可满分；审计 detail 记录 `system / n / rel_err / solution / true`

**方程组库（system_key，后端持真值，前端仅用 config 展示）**：`sys3_basic`（经典教材例题，解 (2,3,-1)）、`sys4_tridiag`（三对角，对角占优，解 (1,-1,2,0.5)）、`sys3_diagdom`（严格对角占优，解 (1,2,3)，雅可比迭代默认题）、`hilbert4`（4x4 Hilbert 病态矩阵，解全 1，教学选主元意义）、`sys6_poisson`（一维泊松方程差分三对角对称正定，解全 1，SOR 迭代默认题）。新增方程组改 `scoring.go` 的 `linSystems` 并同步此处。注意：雅可比迭代仅保证在对角占优系统上收敛（sys3_basic / hilbert4 均不收敛，可作教学反例；sys6_poisson 虽收敛但 114 步超 100 上限，衬托 SOR 加速）。

### L4 jacobi_iter 雅可比迭代法解线性方程组（v0.9，2026-08-27 插队上线）

与 L3 高斯消去构成"直接法 vs 迭代法"对照。命名说明：雅可比是迭代法（x^{(k+1)} = D^-1(b-(L+U)x^{(k)})），规范术语为"雅可比迭代法"。

**config（GET /experiments/:id 下发）**

```json
{ "system_key": "sys3_diagdom",
  "system_desc": "10x + 2y + z = 17; x + 10y + 2z = 27; 2x + y + 10z = 34",
  "A": [[10,2,1],[1,10,2],[2,1,10]], "b": [17,27,34], "x0": [0,0,0],
  "tolerance": 1e-6, "max_iterations": 100, "expected_iterations": 14 }
```

sys3_diagdom 严格对角占优（雅可比必收敛），解恰为整数 (1,2,3)，手工迭代即可满分；expected_iterations=14 为后端实测（x0=零向量，max_i|Δx|<1e-6 停止）。

**readings（POST /progress/submit）**

```json
[ { "solution": [1, 2, 3], "iterations": 14, "x0": [0, 0, 0] } ]
```

- `solution` 必填，长度须等于未知数个数（不匹配/含 NaN/Inf -> 400）；`iterations` 须在 1~max_iterations（缺省不罚分）；`x0` 可选（缺省依次回退 `user_config.x0` / `target.x0` / 零向量；readings/user_config 的 x0 维度错 -> 400，target.x0 换题后维度不符则静默回退零向量）
- **target.method = `jacobi_rel_err`**：**真值解 = gaussSolve 列主元高斯精确解**（真值不落库，收敛时与迭代解一致，且可覆盖不收敛系统）；**期望迭代数 = 后端从学生 x0 模拟雅可比**（停止准则 `max_i |Δx| < tolerance`）
- **迭代效率罚分**：`4 x max(0, expected/2 - 2 - iterations)`。方向与牛顿实验相反：声称迭代数**低于期望一半再留 2 步宽限**才罚（高斯-赛德尔约快一倍属合法实现，不罚），防用直接消去法求出精确解后冒充少量雅可比迭代提交；如实报高迭代数（收敛慢）不罚
- 系统非对角占优（如换 sys3_basic/hilbert4 教学反例）时后端模拟不收敛 -> 无期望迭代数、不罚分，仍按解精度计分
- **user_config**：`system_key` 换库内方程组；或 `A`/`b` 自定义（2~8 阶方阵、对角元须非零、奇异 -> 400）；`x0` 换初值（后端按同初值模拟）
- 审计 detail 记录 `system / n / rel_err / solution / true / x0 / iterations / expected_iterations / penalty`

### L5 sor_iter SOR 逐次超松弛迭代法解线性方程组（v0.11，2026-09-10 插队上线）

与 L14 雅可比构成"基础迭代法 vs 加速迭代法"对照：SOR = 高斯-赛德尔就地更新 + 松弛因子 ω。迭代式 `x^{(k+1)}_i = (1-ω)·x_i + (ω/a_ii)·(b_i - Σ_{j<i} a_ij·x_j^{(k+1)} - Σ_{j>i} a_ij·x_j^{(k)})`，ω=1 退化为高斯-赛德尔，ω∈(0,1) 为低松弛、ω∈(1,2) 为超松弛。

**config（GET /experiments/:id 下发）**

```json
{ "system_key": "sys6_poisson",
  "system_desc": "2x1 - x2 = 1; -x1 + 2x2 - x3 = 0; -x2 + 2x3 - x4 = 0; -x3 + 2x4 - x5 = 0; -x4 + 2x5 - x6 = 0; -x5 + 2x6 = 1",
  "A": [[2,-1,0,0,0,0],[-1,2,-1,0,0,0],[0,-1,2,-1,0,0],[0,0,-1,2,-1,0],[0,0,0,-1,2,-1],[0,0,0,0,-1,2]],
  "b": [1,0,0,0,0,1], "x0": [0,0,0,0,0,0],
  "omega": 1.0, "omega_min": 0.5, "omega_max": 1.95,
  "omega_hint": "omega=1 即高斯-赛德尔；试试 1.3~1.5，观察 61 步被加速到约 19 步",
  "tolerance": 1e-6, "max_iterations": 100, "expected_iterations": 61 }
```

sys6_poisson 为一维泊松方程 -u''=f 差分离散（6 内点）的三对角对称正定阵，解恰为全 1，是 SOR 经典主场：雅可比 114 步（超 max_iterations=100 不收敛）、ω=1 高斯-赛德尔 61 步、最优 ω≈1.4 仅 19 步——学生调 ω 寻优即本关教学目标。`omega_min`/`omega_max`/`omega_hint` 为前端滑块界参数与提示，评分不使用。

**readings（POST /progress/submit）**

```json
[ { "solution": [1, 1, 1, 1, 1, 1], "iterations": 19, "omega": 1.4, "x0": [0,0,0,0,0,0] } ]
```

- `solution` 必填，长度须等于未知数个数（不匹配/含 NaN/Inf -> 400）；`iterations` 须在 1~max_iterations（缺省不罚分）；`omega` 可选（缺省依次回退 `user_config.omega` / `target.omega` / 1.0，取值须在 (0,2) 开区间否则 400--SOR 收敛必要条件）；`x0` 可选（回退链同 L4：`user_config.x0` / `target.x0`（换题后维度不符静默回退零向量）/ 零向量）
- **target.method = `sor_rel_err`**：**真值解 = gaussSolve 列主元高斯精确解**（真值不落库）；**期望迭代数 = 后端从学生 x0 与 ω 模拟 SOR**（高斯-赛德尔式就地更新，`max_i |Δx| < tolerance` 停止）。ω 一律以学生实际提交为准（readings.omega > user_config.omega > target.omega > 1.0），换 ω 后按新口径算期望，防按旧口径虚报
- **迭代效率罚分**：与 L4 同式 `4 x max(0, expected/2 - 2 - iterations)`，防用直接消去法求精确解后冒充少量迭代。**调大 ω 加速属合法**：期望随学生 ω 同步重算，如实报数不罚（ω=1 却报最优 ω 的 19 步会被罚 36 分）；模拟不收敛（ω 过大/系统不合适）则无期望、不罚分，仅按精度计分
- **user_config**：`system_key` 换库内方程组；或 `A`/`b` 自定义（2~8 阶方阵、对角元须非零、奇异 -> 400）；`x0` 换初值、`omega` 换松弛因子（后端按同参数模拟）
- 审计 detail 记录 `system / n / rel_err / solution / true / omega / x0 / iterations / expected_iterations / penalty`

### L6 lagrange_interp 拉格朗日插值（v0.12，2026-09-12 上线）

与前五关的"求根/解方程组"不同，本关是**函数逼近**：给定节点表 (x_i, y_i)，构造拉格朗日插值多项式 P(x) = Σ_i y_i·Π_{j≠i}(x−x_j)/(x_i−x_j)，在求值点 x_eval 处取值。无迭代过程 -> **无迭代效率罚分**，纯插值值精度计分。龙格现象（等距高次插值在区间端部剧烈振荡）为教学观察点：前端画 f 与 P 双曲线对照，P 与 f 的偏离即插值误差。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "sin_quad",
  "problem_desc": "sin x 三点二次插值：节点 0、π/4、π/2 处的函数值构成插值表",
  "func_desc": "f(x) = sin x",
  "nodes_x": [0, 0.7853981633974483, 1.5707963267948966],
  "nodes_y": [0, 0.7071067811865476, 1],
  "x_eval": 0.5235987755982988,
  "x_eval_hint": "P(π/6)：先算三个基函数 L_i(π/6)，再加权求和" }
```

`nodes_x`/`nodes_y`/`x_eval` 为当前题的节点与求值点（与题库一致，前端直接用）；原函数 f 仅供前端画图对照，不下发公式本身（换题时前端按 problem_key 从本地题库取 f）。

**readings（POST /progress/submit）**

```json
[ { "value": 0.517428, "basis": [0.2222, 0.8889, -0.1111] } ]
```

- `value` 必填（= 插值多项式在 x_eval 处的值；缺字段/NaN/Inf -> 400）；`basis` 可选（学生算的各基函数值 L_i(x_eval)，长度须等于节点数，仅记入审计 detail 供教师查计算过程，不参与打分）
- **target.method = `interp_rel_err`**：真值 P(x_eval) 由后端按 target/user_config 的节点**现算**（插值多项式由节点唯一确定，无迭代路径歧义），按 `|value−P|/|P|` 相对误差走默认曲线（`target.bands` 可覆盖）
- **user_config 三种换题方式**：`{"problem_key":"..."}` 换库内题；`{"x_eval":...}` 换求值点；`{"nodes_x":[...],"nodes_y":[...],"x_eval":...}` 自定义节点（2~8 个、x 互异、须同时给 x_eval，后端重算真值）
- **求值点归属规则（v0.12.1 修正）**：`target.x_eval` 只属于 target 自己的题——优先级为 `user_config.x_eval` >（仅未换题时）`target.x_eval` > 新题的库内默认求值点。即 user_config 换了 `problem_key`（或自定义节点）却不带 `x_eval` 时，按**新题**的库内默认求值点评分，不沿用 target 的；前端换题时始终显式携带 `x_eval` 则不受此规则影响
- 审计 detail 记录 `problem / n / x_eval / value / rel_err / true_P / basis`

**插值题库（problem_key，后端持真值，前端另存 f 用于画图）**：`sin_quad`（sin x 三点二次插值，基函数值恰为 2/9、8/9、−1/9 手工可验证，P(π/6)≈0.517428 vs sin(π/6)=0.5）、`sqrt_table`（教材经典开方表，节点 (100,10)/(121,11)/(144,12) 求 √115，P(115)≈10.722756 vs √115≈10.723805）、`runge_5`（龙格现象 f=1/(1+25x²) 五等距节点，P(0.8)≈−0.379310 vs f(0.8)=0.058824——抄原函数值与插值真值差 ~1.15 倍相对误差 -> 0 分，天然防"不插值抄 f 值"）。新增题改 `scoring.go` 的 `interpProblems` 并同步此处与前端 `numlab.js`。

### L7 simpson_integration 复合辛普森积分法（v0.13，2026-09-16 上线）

数值积分：把 [a,b] 分成 n（偶数）段，每两段用过三节点的抛物线（拉格朗日二次）逼近 f 后积分，`S_n = (h/3)·[f(a) + 4·Σ奇下标 f + 2·Σ偶下标 f + f(b)]`，h=(b−a)/n。无迭代过程 -> 无迭代效率罚分，纯辛普森值精度计分。**真值是"方法的输出"S_n 而非精确积分 I**（同 L16 真值=P 而非 f 的设计）：学生正确执行公式即 100 分；直接抄精确积分被截断误差压住（sin_pi 默认 n=2，I=2 vs S₂=2π/3，相对误差 ~4.5% -> ~42 分不过线）。n 加大 S_n 逼近 I 的收敛行为（含 sqrt_x 的端点导数奇异、辛普森失效情形）为教学观察点。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "sin_pi",
  "problem_desc": "辛普森经典验证：∫₀^π sin x dx = 2（精确值已知），先用 n=2 单次辛普森算 S₂ = (π/6)·(f(0) + 4f(π/2) + f(π))",
  "func_desc": "f(x) = sin x",
  "a": 0, "b": 3.141592653589793, "n": 2,
  "n_hint": "n 为偶数子区间数（2~1024），加大 n 观察 S_n 逼近精确积分 2" }
```

`a`/`b`/`n` 为当前题的区间与分段数（与题库一致，前端直接用）；f 本身不下发（换题时前端按 problem_key 从本地题库取 f 画图）。

**readings（POST /progress/submit）**

```json
[ { "value": 2.0943951, "odd_sum": 1, "even_sum": 0 } ]
```

- `value` 必填（= 学生算出的复合辛普森值 S_n；缺字段/NaN/Inf -> 400）；`odd_sum`/`even_sum` 可选（学生算的奇/偶下标节点函数值和，仅记入审计 detail 供教师查计算过程，不参与打分，NaN/Inf -> 400）
- **target.method = `integ_rel_err`**：真值 S_n 由后端按**同一 f、区间、n** 现算（复合辛普森确定性公式，无路径歧义），按 `|value−S_n|/|S_n|` 相对误差走默认曲线（`target.bands` 可覆盖）；精确积分 I 由后端高精度参考积分算出仅记入 detail（`ref_I`），不参与打分
- **user_config 三种覆盖方式**：`{"problem_key":"..."}` 换库内题；`{"a":...,"b":...}` 换积分区间（须成对给出且 a<b）；`{"n":...}` 换分段数（偶数 2~1024，奇数/越界 -> 400）——后端一律按生效参数重算真值 S_n
- **题专属参数归属规则（同 v0.12.1 求值点归属）**：`target.a`/`b`/`n` 只属于 target 自己的题——user_config 换了 `problem_key` 却不带区间/n 时，按**新题**的库内默认区间与分段数评分，不沿用 target 的；前端换题时始终显式携带 `a`/`b`/`n` 则不受此规则影响
- 审计 detail 记录 `problem / a / b / n / h / value / rel_err / true_S / ref_I / odd_sum / even_sum`

**积分题库（problem_key，后端持真值，前端另存 f 用于画图）**：`sin_pi`（∫₀^π sin x dx，I=2 已知可验证，默认 n=2 单次辛普森 S₂=2π/3 手工闭式可验）、`erf_gauss`（f=e^(−x²)，I=(√π/2)·erf(1)≈0.746824 无初等闭式必须数值积分，默认 n=4）、`sqrt_x`（f=√x 端点导数奇异，辛普森四阶收敛假设被破坏收敛慢，默认 n=8 时 S₈≈0.663079 vs I=2/3）。新增题改 `scoring.go` 的 `integProblems` 并同步此处与前端 `numlab.js`。

### L8 aitken_interp 逐次线性插值法 Aitken/Neville（v0.14，2026-09-17 插队上线）

与 L6 拉格朗日插值构成"同一插值多项式、两种计算路径"对照：L6 直接构造基函数加权求和，本关用**逐次线性插值递推**——`P_{i,0} = y_i`，`P_{i,k} = ((x−x_{i−k})·P_{i,k−1} − (x−x_i)·P_{i−1,k−1}) / (x_i − x_{i−k})`（k=1..i），每一步都是相邻低阶结果之间的线性插值，三角表逐列推进到终值 P_{n−1,n−1}。**真值与 L6 数学上相等**（插值多项式由节点唯一确定，单测交叉验证两路计算结果），区别全部在教学过程：每格给出两个低阶值与线性插值权重，学生逐格手算填表。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "sin_quad",
  "problem_desc": "sin x 三点二次插值：与 L16 同一张节点表，改用逐次线性插值（Aitken）递推求 P(π/6)",
  "func_desc": "f(x) = sin x",
  "nodes_x": [0, 0.7853981633974483, 1.5707963267948966],
  "nodes_y": [0, 0.7071067811865476, 1],
  "x_eval": 0.5235987755982988,
  "method_hint": "先算第 1 列：相邻节点的线性插值 P_{i,1}；再由第 1 列线性插值得 P_{2,2}" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 0.517428, "col1": [0.471405, 0.609476] } ]
```

- `value` 必填（= 递推终值 P_{n−1,n−1}，即插值多项式在 x_eval 处的值；缺字段/NaN/Inf -> 400）；`col1` 可选（学生算的第 1 列：相邻节点线性插值结果 P_{i,1}，长度须为节点数−1，仅记入审计 detail 供教师查计算过程，不参与打分）
- **target.method = `aitken_rel_err`**：真值由后端按 target/user_config 节点用 **Neville 递推现算**（题库与 L6 共用 `interpProblems`），按 `|value−P|/|P|` 相对误差走默认曲线（`target.bands` 可覆盖）；龙格题抄原函数值同样 0 分
- **user_config 与 L6 完全同语义**：`{"problem_key":"..."}` 换库内题；`{"x_eval":...}` 换求值点；`{"nodes_x":[...],"nodes_y":[...],"x_eval":...}` 自定义节点（2~8 个、x 互异、须同时给 x_eval）
- **求值点归属规则（同 v0.12.1）**：`target.x_eval` 只属于 target 自己的题——换 `problem_key` 不带 `x_eval` 时按新题库内默认求值点评分，不沿用 target 的
- 审计 detail 记录 `problem / n / x_eval / value / rel_err / true_P / col1`

### L9 romberg_integration 龙贝格积分法（v0.15，2026-09-17 插队上线）

与 L7 复合辛普森构成"同一题库、精度加速"对照：L7 用固定 n 的复合辛普森，本关用**梯形逐次减半 + 理查逊外推**——`T_0 = (b−a)/2·[f(a)+f(b)]`，`T_m = T_{m−1}/2 + h_m·Σf(新节点)`（h_m=(b−a)/2^m，每层只算新增奇数位节点），再逐列外推 `T_i^(j) = (4^j·T_i^(j−1) − T_{i−1}^(j−1))/(4^j − 1)`，终值 T_k^(k)。**外推第 1 列恰为复合辛普森 S_{2^m}**（单测 `TestScoreRombergIntegration_FirstColumnEqualsSimpson` 结构性交叉验证；sin_pi 默认 k=2 时 S_1=2π/3 正是 L7 的 S_2——梯形递推、辛普森、更高阶外推一条链）。**真值=龙贝格表终值 T_k^(k)**（后端按同一 f、区间、k 现算，"真值=方法输出"，同 L7/L8 设计）。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "sin_pi",
  "problem_desc": "sin x 龙贝格求积：梯形逐次减半得 T_0→T_1→T_2，再做两轮理查逊外推求终值（外推第 1 列 S_1=2π/3 正是 L17 的 S_2）",
  "func_desc": "f(x) = sin x",
  "a": 0, "b": 3.141592653589793, "k": 2,
  "method_hint": "T_m = T_{m−1}/2 + h_m·Σf(新节点)；逐列外推 T_i^(j) = (4^j·T_i^(j−1) − T_{i−1}^(j−1))/(4^j − 1)，终值 T_k^(k) 即提交值" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 1.998571, "t_col": [0, 1.570796, 1.896119] } ]
```

- `value` 必填（= 递推终值 T_k^(k)；缺字段/NaN/Inf -> 400）；`t_col` 可选（学生算的梯形列 T_0..T_k，长度须为 k+1，仅记入审计 detail 供教师查计算过程，不参与打分）
- **target.method = `romberg_rel_err`**：真值由后端按 target/user_config 参数用 rombergEval 现算（题库与 L7 共用 `integProblems`），按 `|value−T_k^(k)|/|T_k^(k)|` 相对误差走默认曲线（`target.bands` 可覆盖）
- **防作弊语义（与 L7 有别，写进课内说明）**：抄精确积分 I=2 约 81 分——能过线但非满分（龙贝格收敛快，抄答案与算对只差 0.07% 截断误差；过程真伪由 t_col 审计暴露）；抄 L7 答案 2π/3（= 本表 S_1，漏做第二轮外推）或抄最后一行梯形 T_2（漏做全部外推）均 ~41 分不过线
- **user_config 与 L7 同语义**：`{"problem_key":"..."}` 换库内题；`{"a":...,"b":...}` 换区间（成对，a<b）；`{"k":...}` 换外推深度（1~10）
- **题专属参数归属规则（同 v0.12.1）**：target 的 a/b/k 只属于 target 自己的题——换 `problem_key` 不带 k 时按库内默认 k=2 评分，不沿用 target 的
- 审计 detail 记录 `problem / a / b / k / value / rel_err / true_R / ref_I / t_col`

### L10 euler_explicit 显式欧拉法（v0.16，2026-09-21 插队上线）

常微分方程初值问题 y' = f(x,y)、y(a)=y0 的数值解入门：**欧拉折线递推** `y_{n+1} = y_n + h·f(x_n, y_n)`（前进欧拉，局部截断误差 O(h)）。**真值=方法输出 y_N**（后端按同一 f、初值、区间、步长现算，"真值=方法输出"同 L7/L9 设计；精确解闭式记审计 `ref_Y` 展示截断误差，不参与打分）。与 L11 改进欧拉构成"同题库、精度阶对照"（O(h) vs O(h²)）。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "exp_growth",
  "problem_desc": "y'=y, y(0)=1 的欧拉折线递推：y_{n+1}=y_n+h·f(x_n,y_n)，求 y(1)（默认 n=4，h=0.25）",
  "func_desc": "f(x, y) = y（右端函数与精确解 e^x 对照，见斜率场）",
  "a": 0, "b": 1, "y0": 1, "n": 4,
  "method_hint": "折线递推 y_{n+1} = y_n + h·f(x_n, y_n)（前进欧拉，局部截断误差 O(h)）；stiff_decay 题观察 hλ 超出绝对稳定区间 |1+hλ|≤1 时的振荡发散" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 2.44140625, "y_col": [1, 1.25, 1.5625, 1.953125, 2.44140625] } ]
```

- `value` 必填（= 递推终值 y_N；缺字段/NaN/Inf -> 400）；`y_col` 可选（学生算的逐点列 y_0..y_N，长度须为 n+1，仅记入审计 detail 供教师查计算过程，不参与打分）
- **target.method = `euler_rel_err`**：真值由后端按 target/user_config 参数用 eulerExplicit 现算（题库 `odeProblems`），按 `|value−y_N|/|y_N|` 相对误差走默认曲线（`target.bands` 可覆盖）
- **防作弊语义**：抄精确解 e≈2.71828 与显式欧拉真值 2.44140625 差 ~11% -> ~28 分不过线（O(h) 截断误差拦截）；stiff_decay 题抄精确解 e^{-10}≈4.5e-5 与发散真值 5.0625 差 ~100% -> 0 分
- **user_config**：`{"problem_key":"..."}` 换库内题；`{"a":...,"b":...}` 换区间（成对，a<b）；`{"n":...}` 换步数（1~1024）
- **题专属参数归属规则（同 v0.12.1）**：target 的 a/b/n 只属于 target 自己的题——换 `problem_key` 不带 n 时按库内默认 n 评分，不沿用 target 的
- 审计 detail 记录 `problem / a / b / n / value / rel_err / true_Y / ref_Y / y_col`

**常微分方程题库（odeProblems，前后端逐字同步，均 [0,1] 默认 n=4 h=0.25）**：`exp_growth`（y'=y, y(0)=1，精确解 e^x；显式欧拉 (1+h)^n=1.25^4=2.44140625 全列手算闭式）、`decay_sq`（y'=−2xy, y(0)=1，精确解 e^{−x²}，与 L7 erf_gauss 同源；显式 n=4 全列为 2 的幂分数 y4=0.41015625）、`stiff_decay`（y'=−10y，精确解 e^{−10x}；**刚性绝对稳定性观察题**：hλ=−2.5 时 |1+hλ|=1.5>1 解振荡发散，真值仍确定 (−1.5)^4=5.0625——老实递推照常 100 分）。新增题改 `scoring.go` 的 `odeProblems` 并同步 `numlab.js`。

### L11 euler_improved 改进欧拉法（v0.16，2026-09-21 插队上线）

与 L10 同题库（`odeProblems`）的**预测-校正**两步法：预测 `y_p = y_n + h·f(x_n, y_n)`（显式欧拉探一步），校正 `y_{n+1} = y_n + h/2·[f(x_n,y_n) + f(x_{n+1},y_p)]`（单步梯形公式取显式欧拉做预测），局部截断误差 O(h²)。真值=改进欧拉 y_N（后端现算）；target 默认 exp_growth n=2（两轮预测-校正）。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "exp_growth",
  "problem_desc": "y'=y, y(0)=1 的预测-校正递推：先显式欧拉探一步 y_p，再取梯形平均校正，求 y(1)（默认 n=2，h=0.5）",
  "func_desc": "f(x, y) = y（预测-校正即单步梯形公式，局部截断误差 O(h²)）",
  "a": 0, "b": 1, "y0": 1, "n": 2,
  "method_hint": "预测 y_p = y_n + h·f(x_n,y_n)；校正 y_{n+1} = y_n + h/2·[f(x_n,y_n) + f(x_{n+1},y_p)]；y'=y 时每步恰乘 1+h+h²/2" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 2.640625, "y_col": [1, 1.75, 2.640625] } ]
```

- `value` 必填（= 递推终值 y_N）；`y_col` 可选（长度须为 n+1，同 L20）
- **target.method = `euler_improved_rel_err`**：真值由后端用 eulerImproved 现算，评分与归属规则与 L20 完全一致
- **防作弊语义（与 L20 对照，写进课内说明）**：抄精确解 e 与 n=2 真值 2.640625 差 ~2.9% -> ~49 分不过线；**抄 L20 显式欧拉答案 2.44140625（只做预测漏做校正）差 ~7.5% -> ~34 分不过线**；stiff_decay 上改进欧拉同样发散（每步乘 1+hλ+(hλ)²/2=1.625>1）但值与显式不同（(13/8)^4≈6.9729 vs 5.0625），互抄也拦
- 审计 detail 记录 `problem / a / b / n / value / rel_err / true_Y / ref_Y / y_col`（含方法标注"显式/改进欧拉"与精确解 Desc）

### L12 runge_kutta_ode 四阶龙格-库塔法（v0.17，2026-09-22 上线）

与 L20/L21 同题库（`odeProblems`）的**四阶 Runge-Kutta** 单步递推：`K1=f(x_n,y_n)`、`K2=f(x_n+h/2, y_n+h/2·K1)`、`K3=f(x_n+h/2, y_n+h/2·K2)`、`K4=f(x_n+h, y_n+h·K3)`，加权平均 `y_{n+1} = y_n + h/6·(K1+2K2+2K3+K4)`，全局截断误差 O(h⁴)——与 L20 O(h)、L21 O(h²) 构成精度阶对照。真值=RK4 y_N（后端用 rk4Column 现算，"真值=方法输出"同 L20/L21）；target 默认 exp_growth n=1（单步四阶：K1..K4 = 1、1.5、1.75、2.75 全手算，终值恰 65/24≈2.7083333，是经典教材习题）。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "exp_growth",
  "problem_desc": "y'=y, y(0)=1 的经典四阶 Runge-Kutta 单步递推：K1..K4 四次斜率探测加权平均，求 y(1)（默认 n=1，h=1，单步）",
  "func_desc": "f(x, y) = y（RK4 全局截断误差 O(h⁴)，与 L20/L21 构成精度阶对照 O(h)/O(h²)/O(h⁴)）",
  "a": 0, "b": 1, "y0": 1, "n": 1,
  "method_hint": "K1=f(x_n,y_n)；K2=f(x_n+h/2, y_n+h/2·K1)；K3=f(x_n+h/2, y_n+h/2·K2)；K4=f(x_n+h, y_n+h·K3)；y_{n+1}=y_n+h/6·(K1+2K2+2K3+K4)" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 2.7083333333333335, "y_col": [1, 2.7083333333333335] } ]
```

- `value` 必填（= 递推终值 y_N）；`y_col` 可选（长度须为 n+1，同 L20/L21）
- **target.method = `rk4_rel_err`**：真值由后端用 rk4Column 现算，评分与归属规则（题专属参数 a/b/n 只归 target 自己的题）与 L20/L21 完全一致
- **防作弊语义（与 L20/L21 对照，写进课内说明）**：RK4 收敛快，抄精确解 e 与单步真值 65/24 差仅 ~0.37% -> 约 72 分，能过线但非满分——**过程真伪由 y_col 审计暴露**（L19 龙贝格同款语义）；**抄 L21 改进欧拉答案 2.640625 差 ~2.5% -> ~51 分不过线；抄 L20 显式欧拉答案 2.44140625 差 ~9.8% -> ~30 分不过线**；stiff_decay 单步 RK4 即发散（稳定性函数 |R(−10)|=291>1，终值恰 291），抄精确解 e^{-10} 或抄 L20/L21 发散值（5.0625/6.9729）均 0 分
- 审计 detail 记录 `problem / a / b / n / value / rel_err / true_Y / ref_Y / y_col`（含方法标注"RK4 四阶龙格-库塔"与精确解 Desc）

### L13 adams_pc_ode 阿当姆斯预报-校正法（v0.18，2026-10-01 上线）

与 L20-L22 同题库（`odeProblems`）的**线性多步法**：表头 y_1..y_3 用 RK4 启动（与 L22 同式，表头同阶精度），第 4 步起预报-校正（PECE，每步只校一次）：

- 预报（AB4 外推）：`y^p_{n+1} = y_n + h/24·(55f_n − 59f_{n−1} + 37f_{n−2} − 9f_{n−3})`
- 校正（AM4 内插）：`y_{n+1} = y_n + h/24·(9f(x_{n+1}, y^p) + 19f_n − 5f_{n−1} + f_{n−2})`

全局截断误差 O(h⁴)，与 L20 O(h)、L21 O(h²)、L22 O(h⁴) 构成**单步法/多步法方法族对照**。真值=AB4-AM4 y_N（后端用 adamsPCColumn 现算，"真值=方法输出"同 L20-L22）；target 默认 exp_growth n=4（3 步 RK4 表头 + 1 步 AB4-AM4，经典手算规模：表头 y_1..y_3 = R(1/4)^i，R(h)=1+h+h²/2+h³/6+h⁴/24，终值 2.7182259833）。

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "exp_growth",
  "problem_desc": "y'=y, y(0)=1 的阿当姆斯线性多步法：前 3 步用 RK4 启动做表头，第 4 步起 AB4 外推预报 + AM4 内插校正一次（PECE），求 y(1)（默认 n=4，h=0.25）",
  "func_desc": "f(x, y) = y（AB4-AM4 全局截断误差 O(h^4)，与 L20/L21/L22 单步法构成方法族对照；多步法一次算出 4 个历史斜率的加权组合）",
  "a": 0, "b": 1, "y0": 1, "n": 4,
  "method_hint": "表头 RK4：y_{i+1} = y_i + h/6*(K1+2K2+2K3+K4)；预报 AB4：y^p_{n+1} = y_n + h/24*(55f_n - 59f_{n-1} + 37f_{n-2} - 9f_{n-3})；校正 AM4：y_{n+1} = y_n + h/24*(9f(x_{n+1}, y^p) + 19f_n - 5f_{n-1} + f_{n-2})，只校一次" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 2.718225983274504,
    "y_col": [1, 1.2840169270833333, 1.648699469036526, 2.1169580259162033, 2.718225983274504] } ]
```

- `value` 必填（= 递推终值 y_N）；`y_col` 可选（长度须为 n+1，同 L20-L22）
- **target.method = `adams_rel_err`**：真值由后端用 adamsPCColumn 现算，评分与归属规则（题专属参数 a/b/n 只归 target 自己的题）与 L20-L22 完全一致
- **n 须 4~1024**（3 步 RK4 表头 + 至少 1 步阿当姆斯）：n<4 直接 400（与 L20-L22 的 1~1024 不同，本关下限抬高）
- **防作弊语义（与 L20-L22 对照，写进课内说明）**：四阶方法收敛快，抄精确解 e 与真值 2.7182259833 差仅 ~2e-5 -> 约 93 分、**抄 L22 默认答案 65/24≈2.708333 约 72 分**，均能过线但非满分——同为四阶法数学上难分（过程真伪由 y_col 审计暴露，L19/L22 同款语义）；**抄 L21 改进欧拉答案 2.640625 约 49 分、抄 L20 显式答案 2.44140625 约 30 分均不过线**；stiff_decay（hλ=−2.5）多步法绝对稳定区间远小于 RK4，抄精确解 e^{−10} 直接 0 分、抄 RK4 n=4 表头答案 0.1768 约 9 分也被拦（多步法稳定性观察题）
- 审计 detail 记录 `problem / a / b / n / value / rel_err / true_Y / ref_Y / y_col`（含方法标注"阿当姆斯预报-校正 AB4-AM4 PECE，RK4 表头"与精确解 Desc）

### L14 least_squares_fit 最小二乘拟合（v0.19，2026-10-01 上线，数值分析链收官）

散数据的**最小二乘多项式拟合**（离散逼近，与 L16/L18 插值"过所有点"相对照：LSQ 只使残差平方和最小、不过点）。幂基法方程：

- `Σ_k x_k^{i+j}·c_i = Σ_k x_k^j·y_k`（j=0..m），列主元高斯求解（复用 L13 的 gaussSolve）
- 真值 = 法方程解出的拟合多项式在求值点的值 **P(x_eval)**（后端 lsqFit 现算，"真值=方法输出"同 L7/L9/L16/L20-L23）

**题库 fitProblems（前后端逐字同步）**

- `lin_line`：直线拟合 4 点 (1,2),(2,3.1),(3,4),(4,4.9)，法方程 4a+10b=14、10a+30b=39.8 **手算 a=1.1、b=0.96**，P(3.5)=4.46
- `quad_parabola`：二次拟合 5 点 (0,0.1),(1,1.1),(2,3.9),(3,9.2),(4,15.8)（数据≈x²），系数恰 **27/350、1/28、137/140**，P(2)=711/175；x_eval=2 落在数据点上（LSQ"靠近不过点"的观察点）
- `exp_lnw`：指数模型 **y=a·e^{bx}** 5 点 (0,1.8),(1,2.9),(2,5.2),(3,9.0),(4,16.2)（细菌增长），**ln y 线性化**后拟合 A+Bx，真值=e^{A+2.5B}=6.9103931（跳过线性化直接拟 y → 12 分不过线）

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "lin_line",
  "problem_desc": "散数据直线拟合 y=a+bx：求和表列 x_k、y_k、x_k*y_k、x_k^2 组装法方程（4a+10b=14、10a+30b=39.8 手算可得 a=1.1、b=0.96），解出系数后给出拟合值 P(3.5)；可换题二次拟合/指数模型 ln 线性化",
  "func_desc": "最小二乘：使残差平方和 sum (y_k - P(x_k))^2 最小的 m 次多项式，法方程为 sum_k x_k^{i+j} c_i = sum_k x_k^j y_k（j=0..m）；指数模型 y=a*e^{bx} 先取 ln y 线性化再拟合 A+Bx",
  "m": 1, "x_eval": 3.5,
  "method_hint": "法方程（幂基 m=1）：n*c0 + (sum x)*c1 = sum y；(sum x)*c0 + (sum x^2)*c1 = sum x*y，高斯消去解 c0..c_m 后求 P(x_eval)；m 须 0~min(5, 点数-2)" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 4.46, "coef": [1.1, 0.96] } ]
```

- `value` 必填（= 拟合多项式在 x_eval 处的值；exp_lnw 为回原域的 ŷ(x_eval)=e^{A+B·x_eval}）；`coef` 可选（学生解法方程得的系数 [c_0..c_m]，**长度须 m+1**，仅记审计不打分）
- **target.method = `lsq_rel_err`**：真值由后端按同一数据点、同一 m 现算；m/x_eval 归属规则同 v0.12.1（target 的只属于 target 自己的题，换题/自定义后回落新题库默认；custom 默认 m=1、须显式给 x_eval）
- **m 须 0~min(5, 点数-2)**：上界点数-2 防拟合退化成插值（m=点数-1 时法方程解=插值多项式，防"抄 L16/L18 插值答案"的语义随之失效），5 封顶防法方程病态；越界直接 400
- **防作弊语义（写进课内说明）**：拟合不过点——抄观测 y(3)=4 约 29 分、y(4)=4.9 约 30 分、抄均值 ȳ=3.5 约 16 分均不过线；quad 题 x_eval 落在数据点上，抄观测 3.9→44 分、**抄真函数 x²=4 恰 58 分差线 2 分**、次数用错（直线拟合 6.02）→1 分；exp 题抄观测 5.2/9.0→13/9 分、**不线性化直接拟 y（8.765）→12 分**均不过线；**抄三次插值 P3(3.5)=4.4375 约 69 分能过线但非满分**（插值≠拟合，与 RK4 抄 e 同款"同为合法路径但不该得满分"语义，过程真伪由 coef 审计暴露）
- 审计 detail 记录 `problem / pts / m / x_eval / value / rel_err / true_P / coef`（含方法标注"最小二乘法方程拟合"，指数题加"（指数模型 ln 线性化）"）

### L15 cubic_spline 三次样条插值（v0.20，2026-10-03 上线，用户点名方向插队）

**自然边界三次样条**：过所有节点的分段三次多项式，节点处一阶、二阶导连续（与 L16 拉格朗日单一高次多项式相对照：分段低次治龙格现象）。三弯矩方程（自然边界 M_0=M_{n-1}=0）：

- `mu_i·M_{i-1} + 2M_i + lam_i·M_{i+1} = d_i`（i=1..n-2），`mu_i = h_i/(h_i+h_{i+1})`，`lam_i = 1-mu_i`，`d_i = 6·[(y_{i+1}-y_i)/h_{i+1} - (y_i-y_{i-1})/h_i]/(h_i+h_{i+1})`——三对角且主对角占优，**追赶法**（thomasSolve）求解
- 段 [x_k,x_{k+1}] 上 `S(t) = y_k + B·w + C·w² + D·w³`（w=t−x_k；B=(y_{k+1}-y_k)/h − h(2M_k+M_{k+1})/6，C=M_k/2，D=(M_{k+1}-M_k)/(6h)），与教材 M 式分段三次式数学等价
- 真值 = **S(x_eval)**（后端 splineMoments+splineEvalAt 现算，"真值=方法输出"同 L7/L9/L16/L20-L24）

**题库 splineProblems（前后端逐字同步）**

- `dist_4`（默认）：位移观测数据 4 节点 (0,0),(1,2.5),(2,6),(4,20)，**不等距 h=1,1,2**（练非等距 mu/lambda），三弯矩手算闭式 **M1=15/23、M2=78/23**，真值 S(3)=**559/46**≈12.1521739
- `sin_5`：sin 五等距节点 [0,π/4,π/2,3π/4,π]，**弯矩对称 M1=M3**，真值 S(π/6)≈0.49974——光滑函数上样条逼近极好
- `runge_5`：龙格函数 1/(1+25x²) 五等距节点，**与 L16 拉格朗日同节点同求值点**（三法对照：P(0.8)≈−0.379310 剧烈振荡 vs 样条 S(0.8)≈−0.036340 vs f(0.8)=1/17）；自然边界端段仍下弯（S(0.8)<0，clamped 边界更准但需导数信息——教学点）

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "dist_4",
  "problem_desc": "位移观测数据 (0,0),(1,2.5),(2,6),(4,20)：先列 h_i 与 mu/lambda/d 组装三弯矩方程（不等距 h=1,1,2），追赶法解出弯矩 M1=15/23、M2=78/23，再按段公式求 S(3)=559/46；可换题 sin 五等距节点 / 龙格函数（与 L16 拉格朗日同题对照）",
  "func_desc": "三次样条：过所有节点的分段三次多项式，节点处一阶二阶导连续；自然边界 M_0=M_{n-1}=0 下由三弯矩方程解出弯矩列，段上 S(t)=y_k+B*w+C*w^2+D*w^3",
  "x_eval": 3,
  "method_hint": "三弯矩（自然边界）：mu_i=h_i/(h_i+h_{i+1})，lam_i=1-mu_i，d_i=6*[(y_{i+1}-y_i)/h_{i+1}-(y_i-y_{i-1})/h_i]/(h_i+h_{i+1})；追赶法解 M 后按段公式求 S(x_eval)；样条不外推，x_eval 须在区间内" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 12.152173913043478, "m_col": [0, 0.652174, 3.391304, 0] } ]
```

- `value` 必填（= 样条在 x_eval 处的值 S(x_eval)）；`m_col` 可选（学生解出的弯矩列 [M_0..M_{n-1}]，**长度须=节点数、含端点零弯矩**，仅记审计不打分，同 L18 col1 / L24 coef 惯例）
- **target.method = `spline_rel_err`**：真值由后端按同一节点、同一求值点现算；x_eval 归属规则同 v0.12.1（target 的只属于 target 自己的题，换题/自定义后回落新题库默认；custom 须显式给 x_eval）
- **节点约束**：3~8 个、x **严格递升**（样条节点按 x 从小到大）、x_eval ∈ [x_0, x_{n-1}]——**样条不做外推**（与 L16 拉格朗日可外推不同）；越界/乱序/缺 x_eval 直接 400
- **防作弊语义（写进课内说明）**：dist_4 抄拉格朗日 P(3)=11.5（在 L16 页对同数据做插值）→ **39 分不过线**、抄线性内插 13 → **35 分不过线**、抄观测值全 0 分——只有真解三弯矩才过线；sin_5 光滑逼近好，**抄 sin(π/6)=0.5 → 83 分、抄拉格朗日 P(π/6) → 78 分，过线但非满分**（过程真伪由 m_col 审计暴露，L19 抄 I / L22 抄 e 同款语义）、抄观测 ≤3 分；runge_5 抄 P / 抄 f / 抄观测**全 0 分**；自定义节点抄原函数（如 x² 节点抄 6.25）→ **58 分恰差线 2 分**（自然边界端段偏差）
- 审计 detail 记录 `problem / nodes / x_eval / value / rel_err / true_S / ref_f`（含方法标注"三次样条插值·自然边界三弯矩"；ref_f=原函数值仅审计参考，sin_5/runge_5 有、数据题无）与 `m_col`

### L16 euler_implicit 隐式（后退）欧拉法（v0.21，2026-10-09 上线，用户点名"微分方程数值解"方向）

**隐式（后退）欧拉**：y_{n+1} = y_n + h·f(x_{n+1}, y_{n+1})——未知量 y_{n+1} 同时出现在等式两侧，须解隐式方程（与 L20 显式欧拉 y_{n+1}=y_n+h·f(x_n,y_n) 方向相反，同为 O(h) 一阶法）：

- 题库 odeProblems 均为**线性齐次** y'=a(x)·y（f(x,y)=y·f(x,1)），代入后闭式解出 `y_{n+1} = y_n / (1 − h·a(x_{n+1}))`，a(x)=f(x,1)——无须迭代（非线性 f 才需 Newton/不动点）
- 稳定性函数 **R(hλ) = 1/(1−hλ)**，绝对稳定域 |1−z|≥1 含整个左半平面（**A 稳定**）——与 L20 显式欧拉的圆盘 |1+z|≤1 正好互补；stiff_decay（hλ=−2.5）上 L20 显式 |1+hλ|=1.5 振荡发散 (−1.5)^4=5.0625，隐式 R=2/7 单调收敛 (2/7)^4=**16/2401**≈0.006664——**绝对稳定性的显隐正面对照主场**
- 真值 = **方法输出 y_N**（后端 eulerImplicit 现算，"真值=方法输出"同 L20-L23）

**题库 odeProblems（与 L17 积分/L20-L23 ODE 系列共用）**

- `exp_growth`（默认）：y'=y，y(0)=1，[0,1] n=4——每步除以 (1−h) 恰乘 **4/3**，真值闭式 **(4/3)^4=256/81**≈3.160494（隐式比显式 2.441 离精确解 e 更远：单向低估、从上方压向真解）
- `decay_sq`：y'=−2xy——每步除以 1+2h·x_{n+1}，n=4 真值 ≈**0.344781**
- `stiff_decay`：y'=−10y——A 稳定收敛真值 **16/2401**（见上）；抄精确解 e^{−10} 与抄 L20 显式发散值 5.0625 均 0 分

**config（GET /experiments/:id 下发）**

```json
{ "problem_key": "exp_growth",
  "problem_desc": "y'=y, y(0)=1 的后退欧拉递推：y_{n+1}=y_n+h·f(x_{n+1},y_{n+1})，解隐式方程求 y(1)（默认 n=4，h=0.25，每步除以 1−h 恰乘 4/3）；stiff_decay 题观察 A 稳定：显式发散处隐式单调收敛",
  "func_desc": "f(x, y) = y（隐式方程 y_{n+1}=y_n+h·y_{n+1} 解出 y_{n+1}=y_n/(1−h)，稳定性函数 R=1/(1−hλ)，绝对稳定域含整个左半平面）",
  "a": 0, "b": 1, "y0": 1, "n": 4,
  "method_hint": "后退欧拉 y_{n+1} = y_n + h·f(x_{n+1}, y_{n+1})（局部截断误差 O(h)，与 L20 显式同阶而方向相反：从上方压向精确解）；y'=y 时每步除以 (1−h)；绝对稳定域为 |1−z|≥1（A 稳定），刚性题不发散" }
```

**readings（POST /progress/submit）**

```json
[ { "value": 3.160493827160494, "y_col": [1, 1.3333333333, 1.7777777778, 2.3703703704, 3.1604938272] } ]
```

- `value` 必填（= 学生递推出的终值 y_N）；`y_col` 可选（学生算的逐点列 y_0..y_N，长度须 n+1，仅记审计不打分，L20-L23 同款）
- **target.method = `euler_implicit_rel_err`**：真值由后端按同一 f、初值、区间、步长现算；a/b/n 归属规则同 v0.12.1（target 的只属于 target 自己的题，换题回落新题库默认）
- **user_config**：`problem_key`（换题）/ `a`+`b`（换区间，成对）/ `n`（1~1024）；**边界：h·a(x)=1（如 exp_growth n=1、h=1）时隐式方程无有限解 → 400**
- **防作弊语义（写进课内说明）**：一阶法截断误差 ~14% 全拦截——抄精确解 e → **24 分**、抄 L20 显式答案 2.44140625 → **15 分**、抄 L21 改进欧拉 2.640625 → **21 分**、抄 L22 RK4 65/24 → **23 分**、抄 L23 阿当姆斯 2.7182260 → **24 分**，均不过线；decay_sq 抄精确解 e^{−1} → **36 分**不过线；stiff_decay 抄精确解 / 抄显式发散值 **全 0 分**——只有老实解隐式方程才过线
- 审计 detail 记录 `problem / a / b / n / value / rel_err / true_Y / ref_Y`（含方法标注"隐式欧拉，精确解 …"）与 `y_col`

### L17 lu_decomp LU 分解（Doolittle）解线性方程组（v0.22，2026-10-09 上线，用户点名"继续加实验"方向）

**Doolittle 三角分解**：A = LU——L 单位下三角（消去乘数 m_ik），U 上三角（消去结果）；Ax=b 分两步：前代 Ly=b 得 y，回代 Ux=y 得解。与 L13 列主元高斯消去同族（消去过程数学相同），LU 把乘数存入 L 可**一次分解解多组 b**。本实验**不选主元**：题库五题均可直接分解；自定义矩阵主元≈0 时分解中断 → 400（列主元高斯才稳定——与 L13 的稳定性差异教学点）。

- 真值 = **LU 路径解**（后端 luDecompose+前代+回代现算，"真值=方法输出"同全系列；单测含库内五题 + 自定义阵 LU 与列主元高斯同解的恒等式交叉验证）
- **防作弊语义**：解与 L13 数学同源（同 L18/L16 Neville≡Lagrange 先例：**抄=算对**），过程真伪由 l_col 乘子审计暴露；抄 b 向量 / 零向量 / 分量换位均低分不过线

**题库 linSystems（与 L13/L14/L15 共用，均无需选主元）**

- `sys3_basic`（默认）：2x+y−z=8, −3x−y+2z=−11, −2x+y+2z=−3——解恰 **(2,3,-1)**，乘子列手算闭式 **[-1.5,-1,4]**（l21=−3/2、l31=−2/2、l32=2/0.5），U 对角 2 / 0.5 / −1，前代 y=(8,1,1)
- `sys4_tridiag`：解 (1,-1,2,0.5)；`sys3_diagdom`：解 (1,2,3)；`hilbert4`：病态解全 1（SPD 不选主元仍可分解）；`sys6_poisson`：三对角解全 1（主元 2, 3/2, 4/3, 5/4, 6/5, 7/6）

**config（GET /experiments/:id 下发）**

```json
{ "system_key": "sys3_basic",
  "system_desc": "A=LU 三角分解（Doolittle，不选主元）后分两步求解：前代 Ly=b 得 y，回代 Ux=y 得解。默认题 2x+y-z=8, -3x-y+2z=-11, -2x+y+2z=-3，解恰为整数 (2,3,-1)，乘子列 [-1.5,-1,4] 全手算；换题 hilbert4 观察病态、sys6_poisson 观察三对角",
  "method_hint": "Doolittle 分解 A=LU：消去乘数 m_ik 存入 L（单位下三角），消去结果即 U（上三角）；一次分解可解多组 b（Ax=b -> LUx=b）。注意不选主元时主元≈0 分解中断（列主元高斯才稳定），题库五题均可直接分解" }
```

**readings（POST /progress/submit）**

```json
[ { "solution": [2, 3, -1], "l_col": [-1.5, -1, 4] } ]
```

- `solution` 必填（长度须=未知数个数）；`l_col` 可选审计（L 的乘子按行序 l21,l31,l32,...，长度须 **n(n-1)/2**，仅记 detail 不参与打分——填错乘子但解对仍 100 分，教师端可对照 ref_l 查过程真伪）
- **target.method = `lu_rel_err`**：真值由后端按同一 A、b 做 Doolittle 分解现算；无穷范数相对误差走默认曲线
- **user_config**：`system_key`（换库内方程组）或 `A`/`b`（自定义方程组，2~8 阶方阵，与 L13 完全同款）；**边界：主元≈0（如 [[0,1],[1,0]] 非奇异但首主元为 0）或奇异 → 400**
- 审计 detail 记录 `system / n / rel_err / solution / true`（含方法标注"Doolittle LU 分解·Ly=b 前代·Ux=y 回代"）与 `l_col / ref_l`（ref_l=后端乘子列，供教师对照）

### L18 thomas_solve 追赶法解三对角方程组（v0.23，2026-10-09 上线，用户点名"做追赶法"方向）

**追赶法（Thomas 法）**——LU 分解（L17/L27）的三对角特例，大型三对角阵（热传导差分/三次样条三弯矩）的标准解法。仅存三条对角线，**O(n) 计算量**（L27 满阵 LU 为 O(n³/3)）：

- **追**（Crout 消元）：β₁=b₁；i=2..n：**m_i = a_{i-1}/β_{i-1}**，**β_i = b_i − m_i·c_{i-1}**，d_i ← d_i − m_i·d_{i-1}
- **赶**（回代）：x_n = d_n/β_n；i=n-1..1：**x_i = (d_i − c_i·x_{i+1})/β_i**
- 本实验**不选主元**：对角占优或对称正定阵必安全（题库四题均是）；自定义输入 β≈0 时中断 → 400（与 L13 列主元高斯、L27 LU 的稳定性差异教学点一脉相承）
- 真值 = **追赶法解**（后端 thomasChase 现算，"真值=方法输出"同全系列；单测含题库四题+自定义阵 追赶 vs 列主元高斯 vs LU 三路恒等交叉验证）
- **防作弊语义**：解与 L13/L27 数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 m/beta 双审计暴露；抄 d 向量 / 零向量 / 分量换位均低分不过线

**题库 triSystems（三对角紧凑 a/b/c/d 存储，新题库，均对角占优或对称正定）**

- `tri1_basic`（默认，4 阶）：主对角 (2,3,4,5)、下对角 (1,2,1)、上对角 (1,1,1)、d=(4,10,20,23)——解恰 **(1,2,3,4)**，追出手算闭式 **m=(0.5, 0.8, 0.3125)**、**β=(2, 2.5, 3.2, 4.6875)**、d'=(4, 8, 13.6, 18.75)
- `tri2_poisson`（5 阶）：一维泊松差分 −u''=f（对称正定，sys6_poisson 的 5 内点兄弟）——β=(2, 3/2, 4/3, 5/4, 6/5)，解全 1
- `tri3_small`（3 阶）：严格对角占优入门题——m=(1/4, 2/7)，解恰 (1,1,1)
- `tri4_spline`（6 阶）：三次样条三弯矩系数阵 [2,1; 1,4,1; ...; 1,2]（**L25 呼应：样条内部就是解三对角**），解全 1

**config（GET /experiments/:id 下发）**

```json
{ "system_key": "tri1_basic",
  "system_desc": "三对角方程组的专用解法（LU 分解的特例）：追（Crout 消元）依次算乘数 m 与新主对角 beta 并更新右端，赶（回代）自最后一个未知量倒推解。默认题主对角 (2,3,4,5)、上下对角见页面，解恰 (1,2,3,4)，m 与 beta 全手算；换题 tri2_poisson（对称正定）、tri4_spline（L25 三弯矩阵）",
  "method_hint": "追：beta_1=b_1，m_i=a_{i-1}/beta_{i-1}，beta_i=b_i-m_i*c_{i-1}，d_i<-d_i-m_i*d_{i-1}；赶：x_n=d_n/beta_n，x_i=(d_i-c_i*x_{i+1})/beta_i。仅存三条对角线，O(n) 计算量——大型三对角阵（热传导/样条）的标准解法；不选主元，对角占优或对称正定时必安全" }
```

**readings（POST /progress/submit）**

```json
[ { "solution": [1, 2, 3, 4],
    "m": [0.5, 0.8, 0.3125],
    "beta": [2, 2.5, 3.2, 4.6875] } ]
```

- `solution` 必填（长度须=未知数个数）；`m` 可选审计（乘子 m₂..m_n，长度须 **n-1**）；`beta` 可选审计（新主对角 β₁..β_n，长度须 **n**）——双审计**各全或无**，仅记 detail 不参与打分（填错 m/beta 但解对仍 100 分，教师端可对照 ref_m/ref_beta 查过程真伪）
- **target.method = `thomas_rel_err`**：真值由后端按同一 a/b/c/d 做追赶现算；无穷范数相对误差走默认曲线
- **user_config**：`system_key`（换库内三对角方程组）或 `a`/`b`/`c`/`d`（自定义三对角，长度 n-1/n/n-1/n，**3~8 阶**）；**边界：β≈0（如 b₁=0 非奇异但需选主元，或中途 β₂=1−1·1=0）→ 400**
- 审计 detail 记录 `system / n / rel_err / solution / true`（含方法标注"追赶法·追出 m 与 beta·赶回代得解·三对角 LU 特例"）与 `m / ref_m`、`beta / ref_beta`（后端对照值，供教师查过程）

### L19 cholesky_solve 平方根法解对称正定方程组（v0.24，2026-10-11 上线，"继续补充新的实验"——直接法一章收官）

**平方根法（Cholesky 分解）**——LU 分解（L17/L27）的对称正定特例：A = L·Lᵀ（L 下三角、主对角全正）。对称正定保证根号内恒正、**永不选主元**，计算量约为 LU 的一半（n³/6 vs n³/3）——协方差矩阵/有限元刚度阵的标准解法：

- **分解**（按行 Cholesky–Crout）：**l_ii = sqrt(a_ii − Σ_{k<i} l_ik²)**；**l_ij = (a_ij − Σ_{k<j} l_ik·l_jk)/l_jj**（j<i）
- **前代** Ly=b：y_i = (b_i − Σ_{k<i} l_ik·y_k)/l_ii；**回代** Lᵀx=y：x_i = (y_i − Σ_{k>i} l_ki·x_k)/l_ii
- 真值 = **平方根法解**（后端 choleskySolve 现算，"真值=方法输出"同全系列；单测含题库四题+自定义阵 **LLᵀ 重建恒等式** 与 平方根法 vs 列主元高斯 vs LU 三路同解交叉验证）
- **防作弊语义**：解与 L13/L27/L28 数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 L/y 双审计暴露；抄 b 向量 / 零向量 / 分量换位均低分不过线

**题库 spdSystems（对称正定四题，L/y/x 全整数，课堂手算友好）**

- `spd2_min3`（默认，3 阶）：min(i,j) 阵 [[1,1,1],[1,2,2],[1,2,3]]，b=(6,11,14)——**L 恰为下三角全 1**，前代 y=**(6,5,3)**，解恰 **(1,2,3)**
- `spd1_2x2`（2 阶）：[[4,2],[2,5]]，b=(6,7)——L=**[[2,0],[1,2]]**，y=(3,2)，解恰 (1,1)
- `spd3_tridiag3`（3 阶三对角）：[[4,2,0],[2,5,2],[0,2,10]]，b=(6,9,12)——**L28 呼应：三对角 SPD 追赶法亦安全**，L=[[2,0,0],[1,2,0],[0,1,3]]，y=(3,3,3)，解恰 (1,1,1)
- `spd4_min4`（4 阶）：min(i,j) 阵，b=(4,7,9,10)——L=下三角全 1，y=**(4,3,2,1)**，解全 1

**config（GET /experiments/:id 下发）**

```json
{ "system_key": "spd2_min3",
  "system_desc": "对称正定方程组的专用解法（LU 分解的特例）：A = L·Lᵀ，按行依次算出下三角因子 L（主对角全正），再前代 Ly=b、回代 Lᵀx=y。默认题 min(i,j) 阵 3 阶，L 恰为下三角全 1，y=(6,5,3)，解恰 (1,2,3) 全手算；换题 spd1 入门 2 阶、spd3 三对角（L28 呼应）、spd4 四阶",
  "method_hint": "分解：l_ii=sqrt(a_ii-Σ_{k<i} l_ik²)，l_ij=(a_ij-Σ_{k<j} l_ik·l_jk)/l_jj（j<i）；前代 y_i=(b_i-Σ_{k<i} l_ik·y_k)/l_ii；回代 x_i=(y_i-Σ_{k>i} l_ki·x_k)/l_ii。对称正定保证根号内恒正、永不选主元，计算量约为 LU 一半——协方差矩阵/有限元刚度阵的标准解法" }
```

**readings（POST /progress/submit）**

```json
[ { "solution": [1, 2, 3],
    "L": [[1, 0, 0], [1, 1, 0], [1, 1, 1]],
    "y": [6, 5, 3] } ]
```

- `solution` 必填（长度须=未知数个数）；`L` 可选审计（n 阶方阵，对照取**下三角含主对角 n(n+1)/2 个元**，按行序 l11,l21,l22,...）；`y` 可选审计（前代中间向量，长度须 **n**）——双审计**各全或无**，仅记 detail 不参与打分（填错 L/y 但解对仍 100 分，教师端可对照 ref_L/ref_y 查过程真伪）
- **target.method = `cholesky_rel_err`**：真值由后端按同一 A、b 做 LLᵀ 分解现算；无穷范数相对误差走默认曲线
- **user_config**：`system_key`（换库内方程组）或 `A`/`b`（自定义方程组，**2~5 阶对称方阵**）；**边界：非对称 → 400，根号内≤0（不定阵 [[1,2],[2,1]] 或半正定奇异阵 [[1,1],[1,1]]）→ 400**
- 审计 detail 记录 `system / n / rel_err / solution / true`（含方法标注"平方根法·LLᵀ 分解·前代 Ly=b·回代 Lᵀx=y·对称正定专属"）与 `L / ref_L`、`y / ref_y`（后端对照值，供教师查过程）

## 附：物理实验批次（v0.7 恢复，v0.10 下线）

牛顿环（level 1）与示波器（level 2）曾按《开发任务讲解》PDF 恢复为交付实验（v0.7，2026-08-25）。v0.10（2026-09-09）随项目聚焦数值分析实验链而**整体下线**：migration `011_remove_ring_oscilloscope.sql` 删除 experiments/levels/user_progresses/operation_logs 对应行并重排解锁链；`scoring.go` 评分器与 `scorers` 注册、前端 app.json/index.js 页面注册同步移除（前端页面文件按项目惯例留在磁盘不打包）。历史接口格式（newton_ring 的 least_squares_R 拟合评分、oscilloscope 的 param_match 通道评分）见契约 v0.7-v0.9 归档版本。
