# backend - 大学物理实验 3D 模拟小程序后端

阶段 0 骨架 + 阶段 1 全量接线 + M2 中间件 + 契约 v0.4 补齐 + v0.5 滑块修复+审计补全 + v0.6-v0.9 数值分析实验批次 + v0.10 物理实验批次下线 + v0.11 SOR 迭代上线 + v0.12-v0.16 插值/积分/欧拉批次 + v0.17 龙格-库塔上线 + v0.18 阿当姆斯预报-校正上线 + v0.19 最小二乘拟合上线 + v0.20 三次样条插值上线 + v0.21 隐式欧拉法上线 + v0.22 LU 分解解线性方程组上线 + v0.23 追赶法解三对角方程组上线 + v0.24 平方根法解对称正定方程组上线：Gin + GORM + MySQL + viper + zap，已验证全链路（HTTP -> 路由 -> middleware -> controller -> service -> repository -> MySQL -> JSON）。

文档：`docs/api-contract.md`（接口契约 v0.24，改动需全组知晓）、`docs/database-design.md`（数据库设计）。

## 目录结构

```
cmd/server/main.go     # 入口：加载配置 -> 连库 -> AutoMigrate 7 表 -> seed -> 装配依赖 -> 起服务
cmd/server/seed.go     # 启动期幂等种子数据：dev 账号 / 示范班级（实验/关卡由 migration 自管）
internal/
  config/              # viper 读 configs/config.yaml
  router/              # Gin 路由注册（全部 12 接口 + 鉴权 + 角色守卫）
  middleware/          # JWT 鉴权 / RequireRole 角色守卫
  controller/          # HTTP 入参出参，不碰 SQL
  service/             # 业务逻辑（scoring.go 注册表模式：5 个数值分析实验评分器）
  repository/          # 数据访问（GORM）
  model/               # 数据库模型（7 张表 + 视图 + JSON 类型）
  pkg/                 # jwt / wechat / response
migrations/            # SQL 初始化脚本（001 库+账号，002 七表+种子）
configs/config.yaml    # 开发配置（端口/数据库/JWT/微信）
```

## 运行

```bash
cd backend
go build -o server.exe ./cmd/server
./server.exe          # 监听 :8080
```

服务启动时自动 `AutoMigrate` 全部 7 张表，并幂等写入种子数据（dev 账号、示范班级）。实验/关卡数据由 `migrations/` SQL 自管。MySQL 不可用时降级为仅 `/ping` 可用。

## 当前接口

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /ping | - | 健康检查 `{"message":"pong"}` |
| POST | /auth/login | - | 微信登录（`code` 以 `dev_` 开头走后门） |
| GET | /auth/me | JWT | 当前用户 |
| PUT | /users/:id | JWT（本人） | 补全学籍 |
| GET | /users/:id | - | 阶段0烟雾接口 |
| GET | /classes | JWT | 班级列表（按角色） |
| POST | /classes | teacher/admin | 建班 |
| GET | /classes/:id | 本班教师/admin/本班学生 | 班级详情（含成员列表）v0.4 |
| PUT | /classes/:id | 本班教师/admin | 班级改名 v0.4 |
| DELETE | /classes/:id | 本班教师/admin | 删班 |
| POST | /classes/:id/members | 本班教师/admin | 加成员 |
| DELETE | /classes/:id/members/:userId | 本班教师/admin | 移成员 v0.4 |
| GET | /levels | JWT | 关卡列表（个性化 status） |
| GET | /experiments | JWT | 实验元数据列表（不含 config/target）v0.4 |
| GET | /experiments/:id | JWT | 实验配置（:id 是 level_id） |
| GET | /progress/mine | JWT | 我的进度 |
| GET | /progress/class/:classId | 本班教师/admin | 班级成绩 |
| POST | /progress/submit | student | 提交读数评分 |
| GET | /admin/operation-logs | admin | 审计日志查询（分页+过滤） |
| GET | /audit/mine | JWT | 学生查自己的审计日志 v0.4 |

## 中间件（M2）

- **JWT 鉴权 + 角色守卫**：`middleware.Auth` 注入 uid/role，`RequireRole` 守卫敏感接口
- **限流**（内存令牌桶）：`/auth/login` 按 IP、`/progress/submit` 按 user_id；超限 429 + Retry-After。配置见 `config.rate_limit`
- **CORS**：`mode=debug` 放行所有 Origin；`mode=release` 收紧到 `config.cors.allow_origins` 白名单；OPTIONS 预检 204

## 开发后门开关

`config.allow_dev_backdoor`（默认 `true`）：`true` 时 `code` 以 `dev_` 开头走内置账号签发 token、`POST /login` 别名可用。`false` 时 `dev_` code 被拒(400)、别名不注册、也不当作真 code 去请求微信。上线前设 `false` 并填好 `wechat.appid/secret`。

## 数据库

- 库：`physics_lab`（utf8mb4）；账号：`physics` / `physics123`（非 root）
- 手动初始化：用 root 依次执行 `migrations/001_init.sql`、`migrations/002_core_tables.sql`（均幂等）
  - 注意 Windows 下 mysql 客户端加 `--default-character-set=utf8mb4`，否则中文乱码报错 1366
- 亦可不跑 SQL：服务启动时 `AutoMigrate` 自动建表、`seedAll` 自动填数据

## 下一步

1. 接 Apifox 跑一遍 18 接口联调（dev 后门即可全链路验证）
2. 真机联调：`config.yaml` 填微信 appid/secret；前端 `app.js` 的 `apiBaseUrl` 改为后端电脑 LAN IP
3. 评分调参：`scoring.go` 的 `scoreFromRelErr` 乘数与各实验 `pass_score`（在 002 种子 / seed.go 里）
4. 上线前：`allow_dev_backdoor: false`（自动连带移除 `/login` 别名）、`mode: release`、CORS 白名单收紧到正式域名

## v0.5 变更摘要（2026-08-12）

1. **滑块真值修复**：`POST /progress/submit` 新增可选 `user_config` 字段，后端按前端实际实验参数动态生成评分 target
2. **审计日志补全**（M6 验收）：登录写 `login`、班级操作写 `class.create`/`class.delete`/`class.rename`/`class.member.add`/`class.member.remove`
3. 前端 `newton-ring.js` / `pendulum.js` 提交时携带 `user_config`
4. **数据校验增强**（阶段5）：评分层拒绝 NaN/Inf/负值读数；班级名 trim+长度校验（≤64）；学籍补全 name/student_no 长度校验
5. **审计日志复合索引**（数据组）：`idx_user_time(user_id, created_at)` + `idx_action_time(action, created_at)`，GORM model + migration 同步

## v0.6 变更摘要（2026-08-25）

1. **数值分析实验批次启动**（后端+数据组）：评分框架改注册表模式（`scoring.go` 的 `scorers` map），此后新增实验四件套：评分函数 + 注册一行 + migration 数据 + 契约附录登记
2. **L1 二分法求根上线**（experiment/level id 11）：`target.method=root_rel_err`，真值根由后端按 `func_key`+`interval` 现算（不落库）；`user_config` 可覆盖 `func_key`/`interval` 防背答案；readings/打分曲线见契约附录
3. 新增 `migrations/006_add_numerical_analysis.sql`（幂等）。编号约定：物理实验保留 1-10，数值分析从 11 起
4. **L2 牛顿迭代法求根上线**（experiment/level id 12，前置 L11）：`target.method=newton_rel_err`，后端按学生 x0 模拟牛顿（`|x_{n+1}-x_n|<tol` 停止）得真值根与期望迭代数；多根函数按实际收敛路径评分；迭代效率罚分 `4 x max(0, iterations-expected-2)` 防二分法冒充；migration `007_add_newton_iter.sql`
5. 后续排队：拉格朗日插值 / 复合辛普森积分 / 最小二乘拟合 / RK4 解 ODE（清单见契约附录）

## v0.7 变更摘要（2026-08-25）

1. **按《开发任务讲解》恢复 PDF 交付实验**（交付物 #4"至少 2 个完整可玩实验"）：牛顿环（experiment/level id 1）+ 示波器（id 2），readings 与前端页面代码逐一对齐，migration `008_restore_physics_experiments.sql`（幂等）
2. **打分曲线参数化**：`target.bands` `[[relErr,score],...]`（首档下满分/末档上 0 分/对数插值），缺省回退数值分析默认曲线；物理实验按读数容差量级标定（0.5%~2% -> 100~90），数据组调参不用改代码
3. **解锁链重排（物理优先）**：L1 牛顿环 -> L2 示波器 -> L11 二分法 -> L12 牛顿迭代
4. 牛顿环评分：`r²=kλR` 过原点最小二乘拟合 R（`least_squares_R`，至少 3 组读数，user_config 覆盖 λ/R）；示波器评分：CH1/CH2 的 A/f 四参数平均（`param_match`，须全覆盖通道）
5. **关卡解锁链服务端强制**（M3 验收补全）：前置关卡未过关提交 -> 403（此前仅前端隐藏锁定关卡，直调接口可跳关；回归测试 `TestSubmitLockedLevelForbidden`）

## v0.8 变更摘要（2026-08-26）

1. **数值分析 L3 高斯消去法解线性方程组上线**（experiment/level id 13，前置 L12，插队：插值/积分批次仍在排队）：`target.method=linear_rel_err`，后端用列主元高斯消去法现算真值解（真值不落库），与学生 `solution` 向量按无穷范数相对误差打分；migration `009_add_gauss_elim.sql`（幂等）
2. **user_config 支持两种换题**：`system_key` 换库内方程组（sys3_basic 经典例题 / sys4_tridiag 三对角 / hilbert4 病态矩阵教学选主元），或自定义 `A`/`b`（2~8 阶方阵、元素有限；奇异矩阵 -> 400）
3. gauss_elim 评分同样支持 `target.bands` 覆盖默认曲线；readings 校验：solution 长度须等于未知数个数、不含 NaN/Inf
4. 契约升 v0.8：§12 user_config 表新增 gauss_elim 行，附录新增 L3 章节，解锁链延长为 L1 牛顿环 -> L2 示波器 -> L11 二分 -> L12 牛顿 -> L13 高斯

## v0.9 变更摘要（2026-08-27）

1. **数值分析 L4 雅可比迭代法解线性方程组上线**（experiment/level id 14，前置 L13，插队：插值/积分批次仍在排队）：`target.method=jacobi_rel_err`，真值解=gaussSolve 高斯精确解（不落库），期望迭代数=后端从学生 x0 模拟雅可比（max_i|Δx|<tol 停止）；migration `010_add_jacobi_iter.sql`（幂等）
2. **迭代效率罚分（方向与牛顿实验相反）**：`4 x max(0, expected/2 - 2 - iterations)`，防直接消去法求精确解后冒充少量迭代提交；高斯-赛德尔约快一倍属合法不罚；模拟不收敛（非对角占优）则无期望、不罚分
3. 新增方程组 `sys3_diagdom`（严格对角占优，解恰为整数 (1,2,3)，雅可比默认题，实测 14 步收敛）；user_config 支持 system_key / 自定义 A/b（对角元须非零）/ x0（换题后 target.x0 维度不符静默回退零向量）
4. 与 L13 高斯消去构成"直接法 vs 迭代法"对照；契约升 v0.9（§12 表 + 附录 L4 章节，解锁链延长至 L14）

## v0.10 变更摘要（2026-09-09）

1. **删除物理实验批次全部数据**：牛顿环（experiment/level id 1）与示波器（id 2）下线，migration `011_remove_ring_oscilloscope.sql`（幂等）删除 experiments/levels/user_progresses/operation_logs 对应行
2. **解锁链重排**：L11 二分法(order 1，链头无前置) -> L12 牛顿迭代(order 2) -> L13 高斯消去(order 3) -> L14 雅可比迭代(order 4)；物理实验 id 1-10 空置保留
3. **代码同步清理**：`scoring.go` 删除 `newton_ring`/`oscilloscope` 评分器与注册行（保留 `target.bands` 机制供数值实验用）；scoring_test/integration_test/v04_test 夹具改用存活实验；前端 app.json/index.js 移除两页注册（页面文件按惯例留盘不打包）
4. 契约升 v0.10：各接口示例与 §12 user_config 表清理物理引用，"物理实验批次"附录下线（历史格式见 v0.7-v0.9 归档）

## v0.11 变更摘要（2026-09-10）

1. **数值分析 L5 SOR 逐次超松弛迭代法解线性方程组上线**（experiment/level id 15，前置 L14，插队：插值/积分批次仍在排队）：`target.method=sor_rel_err`，真值解=gaussSolve 高斯精确解（不落库），期望迭代数=后端从学生 x0 与 ω 模拟 SOR（高斯-赛德尔式就地更新，max_i|Δx|<tol 停止）；migration `012_add_sor_iter.sql`（幂等）
2. **新增方程组 `sys6_poisson`**（一维泊松方程差分三对角对称正定，解全 1）：SOR 经典主场--雅可比 114 步（超 100 上限不收敛）、ω=1 高斯-赛德尔 61 步、最优 ω≈1.4 仅 19 步，学生调 ω 寻优即教学目标
3. **ω 以学生实际提交为准**（readings.omega > user_config.omega > target.omega > 1.0，须在 (0,2) 开区间否则 400）：换 ω 后按新口径算期望迭代数；罚分与 L4 同式防直接法冒充，但调大 ω 加速属合法（期望随 ω 同步重算，如实报数不罚）
4. 前端新增 `sor-iter` 实验页（含 ω 滑块）并注册 app.json；契约升 v0.11（§12 表 + 附录 L5 章节 + 方程组库补 sys6_poisson）

## v0.12 变更摘要（2026-09-12）

1. **数值分析 L6 拉格朗日插值上线**（experiment/level id 16，前置 L15，解锁链 order 6）：`target.method=interp_rel_err`，真值=插值多项式 P(x_eval)（由节点唯一确定，后端现算不落库），按 |value−P|/|P| 相对误差走默认曲线；无迭代过程 -> 无迭代罚分；migration `013_add_lagrange_interp.sql`（幂等）
2. **新增插值题库**（`interpProblems`，三题）：`sin_quad`（sin 三点二次插值，基函数值 2/9、8/9、−1/9 手工可验证）、`sqrt_table`（教材开方表经典，P(115)≈10.722756 vs √115≈10.723805）、`runge_5`（龙格现象题，P(0.8)≈−0.379310 与 f(0.8)=0.058824 差 ~1.15 倍相对误差——不插值抄原函数值直接 0 分，天然防作弊）；原函数仅供前端画图对照，不进后端数据结构
3. **readings 格式**：`{value, basis?}`——value 必填（插值多项式在求值点的值），basis 可选（各基函数值 L_i(x_eval)，仅审计不参与打分）；user_config 可换题/换求值点/自定义节点（2~8 个互异，须带 x_eval）
4. 9 个单测（`TestScoreLagrangeInterp_*`：闭式解交叉验证/相对误差曲线/龙格反作弊/换题/自定义节点/x_eval 覆盖/basis 审计/坏输入）；前端新增 `lagrange-interp` 页（f/P 双曲线 canvas + 节点散点 + 基函数逐步计算）并注册；契约升 v0.12（§8/§12 表 + 附录 L6 章节 + 插值题库登记）

## v0.12.1 变更摘要（2026-09-16）

1. **L16 拉格朗日插值真库 e2e 全部通过**：migration 013 入库；dev_student 提交 sin_quad 正确值（100，basis 记审计）、换 runge_5 抄 f(0.8)（0 分，反作弊成立）、runge_5 真值（100）、换 sqrt_table（100）、自定义节点 x²（100）、3 类坏输入（400）；/progress/mine 6/6 全过；审计 detail 记录 problem/n/x_eval/value/rel_err/true_P/basis
2. **求值点归属修正（e2e 中发现的评分 bug）**：`target.x_eval` 仅属于 target 自己的题——user_config 换 `problem_key`/自定义节点却不带 `x_eval` 时，改用新题的库内默认求值点（此前沿用 target 的：换 runge_5 后真值错按 sin_quad 的 π/6 计算，抄 f 值误判 14 分而非 0 分）；前端换题始终显式带 `x_eval` 不受影响。附回归测试 `TestScoreLagrangeInterp_TargetXEvalOnlyForTargetProblem`（10 个 L6 单测全绿）
3. **前端组组件化升级合入（2026-09-15 交付）**：新增共享组件 `conv-plot`（收敛曲线/分量轨迹/三维空间三标签，历史运行归档对比）、`sol-bars`（解向量柱图回代逐根长出）与 `utils/plane3d.js`（三元方程组三维平面软投影）；高斯/雅可比/SOR 三页接入（纯增量，提交链未动）

## v0.13 变更摘要（2026-09-16）

1. **数值分析 L7 复合辛普森积分法上线**（experiment/level id 17，前置 L16，解锁链 order 7）：`target.method=integ_rel_err`，**真值=同一 f、区间、n 下后端现算的复合辛普森值 S_n**（"真值=方法输出"，同 L16 真值=P 的设计；精确积分 I 由高精度参考积分算出仅记审计 detail `ref_I`，不参与打分）；migration `014_add_simpson_integration.sql`（幂等）
2. **天然反抄设计**：真值是 S_n 而非精确积分 I——直接抄 I 被截断误差压住（sin_pi 默认 n=2，I=2 vs S₂=2π/3，相对误差 ~4.5% -> ~42 分不过线）；正确执行公式即 100 分
3. **新增积分题库**（`integProblems`，三题）：`sin_pi`（∫₀^π sin x dx，I=2 已知可验证，n=2 单次辛普森 S₂=2π/3 闭式可验）、`erf_gauss`（∫₀¹ e^(−x²) dx 无初等闭式必须数值积分，n=4）、`sqrt_x`（f=√x 端点导数奇异、辛普森四阶收敛失效的适用性观察题，n=8 时 S₈≈0.663079 vs I=2/3）
4. **readings 格式**：`{value, odd_sum?, even_sum?}`——value 必填（学生算出的 S_n），odd_sum/even_sum 可选（奇/偶下标节点和，仅审计不参与打分）；user_config 可换题/换区间（a+b 成对）/换 n（偶数 2~1024）；**target 的题专属参数（a/b/n）不跨题沿用**（v0.12.1 求值点归属规则同款，换题后回落新题库内默认）
5. 9 个单测（`TestScoreSimpsonIntegration_*`：闭式解交叉验证/相对误差曲线/抄 I 陷阱/换题/换 n/换区间/奇偶和审计/坏输入/target 参数归属）；前端新增 `simpson-integration` 页（f 曲线+分段抛物线 canvas + 积分阴影 + 分点表，求和与加权留学生手算）并注册；契约升 v0.13（§8/§12 表 + 附录 L7 章节 + 积分题库登记，排队表清理重复行）
6. **真库 e2e 全部通过（2026-09-17）**：migration 014 入库；dev_student 提交 sin_pi 正确 S₂（100，odd/even_sum 记审计）、抄精确积分 I=2（42 分不过线，反作弊成立且与理论值吻合）、换 erf_gauss（100）、换 sqrt_x（100）、sin_pi 自定义 n=8（100）、3 类坏输入（400）；/progress/mine **7/7 全过**；审计 detail 记录 problem/a/b/n/h/value/rel_err/true_S/ref_I/odd_sum/even_sum

## v0.14 变更摘要（2026-09-17）

1. **数值分析 L8 逐次线性插值法 Aitken/Neville 上线**（experiment/level id 18，前置 L17，解锁链 order 8，用户点名插队）：`target.method=aitken_rel_err`，真值=Neville 递推终值 P_{n−1,n−1}（后端现算不落库）；migration `015_add_aitken_interp.sql`（幂等）
2. **与 L6 构成"同一插值多项式、两种计算路径"对照**：题库直接复用 L6 的 `interpProblems`（sin_quad / sqrt_table / runge_5），真值与 L6 数学上相等（插值多项式由节点唯一确定），新增 `TestScoreAitkenInterp_NevilleEqualsLagrange` 交叉验证两路计算一致到 1e-12；区别在教学过程——每步都是相邻低阶结果的线性插值逐列推进
3. **readings 格式**：`{value, col1?}`——value 必填（递推终值），col1 可选（第 1 列相邻节点线性插值结果 P_{i,1}，长度=节点数−1，仅审计不参与打分）；user_config 与 L6 完全同语义（换题/换求值点/自定义节点）；求值点归属规则同 v0.12.1；龙格题抄 f 值同样 0 分
4. 10 个单测（`TestScoreAitkenInterp_*`：两路交叉验证/sin_quad 闭式解（终值 (4√2−1)/9 + 第 1 列 √2/3、(2√2−1)/3）/相对误差曲线/龙格反作弊/换题/自定义节点/x_eval 覆盖/col1 审计/坏输入/target 求值点归属）；前端新增 `aitken-interp` 页（三角表逐格递推演示，每格展示两个低阶值与线性插值权重，填表留学生）并注册；契约升 v0.14（§8/§12 表 + 附录 L8 章节）

## v0.15 变更摘要（2026-09-17）

1. **数值分析 L9 龙贝格积分法 Romberg 上线**（experiment/level id 19，前置 L18，解锁链 order 9，用户点名插队）：`target.method=romberg_rel_err`，真值=龙贝格表终值 T_k^(k)（后端按同一 f、区间、k 现算不落库，"真值=方法输出"）；migration `016_add_romberg_integration.sql`（幂等）
2. **与 L7 构成"同一题库、精度加速"对照**：题库复用 L7 的 `integProblems`（sin_pi / erf_gauss / sqrt_x），新增 `rombergTColumn`（梯形逐次减半，每层只算新增奇数位节点）与 `rombergEval`（原地逐列理查逊外推）；**外推第 1 列恰为复合辛普森 S_{2^m}**（`TestScoreRombergIntegration_FirstColumnEqualsSimpson` 对三道题结构性交叉验证，sin_pi 默认 k=2 时 S_1=2π/3 正是 L7 的 S_2）
3. **readings 格式**：`{value, t_col?}`——value 必填（递推终值 T_k^(k)），t_col 可选（梯形列 T_0..T_k，长度=k+1，仅审计不参与打分）；user_config 与 L7 同语义（换题/换区间/换外推深度 k=1~10）；题专属参数（a/b/k）归属规则同 v0.12.1；**防作弊语义与 L7 有别**：抄精确积分 I=2 约 81 分（能过线但非满分，龙贝格收敛快；过程真伪由 t_col 审计暴露），抄 L7 答案 2π/3 或最后一行梯形 T_2 均 ~41 分不过线
4. 11 个单测（`TestScoreRombergIntegration_*`：全表闭式/第 1 列=辛普森交叉验证/精确解（含 t_col 审计）/相对误差曲线/三类抄袭陷阱/换题（erf 闭式+sqrt 锚点）/k 覆盖（S_3=L17 的 S₈）/换区间/t_col 审计/坏输入/target 参数归属）；前端新增 `romberg-integration` 页（梯形逐层减半可视化 + 三角表逐列外推卡）并注册；契约升 v0.15（§8/§12 表 + 附录 L9 章节）

## v0.16 变更摘要（2026-09-21）

1. **数值分析 L10/L11 欧拉法解常微分方程初值问题双关上线**（experiment/level id 20/21，前置 L19/L20，解锁链 order 10/11，用户点名）：L20 显式（前进）欧拉 `euler_rel_err` + L21 改进欧拉（预测-校正/Heun）`euler_improved_rel_err`；真值=方法输出 y_N（后端按同一 f、初值 y0、区间、步长 n 现算不落库；精确解闭式记审计 ref_Y 不打分）；migration `017_add_euler_ode.sql`（幂等，双关）
2. **新增常微分方程题库 `odeProblems`**（前后端逐字同步，均 [0,1] 默认 n=4 h=0.25）：`exp_growth`（y'=y，精确解 e^x，显式 (1.25)^4=2.44140625 全列闭式，改进每步乘 1+h+h²/2）、`decay_sq`（y'=−2xy，精确解 e^{−x²}，f 显含 x,y）、`stiff_decay`（y'=−10y，**刚性绝对稳定性观察题**：hλ=−2.5 时两法均在稳定区间外振荡发散、真值仍确定（显式 (−1.5)^4=5.0625、改进 (13/8)^4≈6.9729），抄精确解 e^{−10} 直接 0 分）
3. **readings 格式**：`{value, y_col?}`——value 必填（递推终值 y_N），y_col 可选（逐点列 y_0..y_N，长度=n+1，仅审计不参与打分）；user_config 换题/换区间/换步数 n=1~1024；题专属参数（a/b/n）归属规则同 v0.12.1；**防作弊语义（L20/L11 对照）**：L20 抄精确解 e ~28 分、L21 抄精确解 ~49 分、**L21 抄 L20 显式答案（漏做校正）~34 分**均不过线
4. 16 个单测（`TestScoreEuler*`：双法闭式与 (1+h)^n / (1+h+h²/2)^n 恒等式交叉验证/精确解（含 y_col 审计）/相对误差曲线/抄袭陷阱（含 stiff 发散题与 L21 抄 L20 答案）/换题/换 n/换区间/y_col 审计/坏输入/target 参数归属）；前端新增 `euler-explicit` / `euler-improved` 两页（斜率场 + 折线递推分步表，改进页预测-校正双列）并注册；契约升 v0.16（§12 表 + 附录 L10/L11 章节 + odeProblems 题库登记 + 链延 order 11）

## v0.17 变更摘要（2026-09-22）

1. **数值分析 L12 四阶龙格-库塔法上线**（experiment/level id 22，前置 L21，解锁链 order 12，难度 4 为链上最高，原排队表末位按序上线）：`runge_kutta_ode`，target.method=`rk4_rel_err`；K1..K4 四次斜率探测加权平均 `y_{n+1}=y_n+h/6·(K1+2K2+2K3+K4)`，全局截断误差 O(h⁴)——与 L20 O(h)、L21 O(h²) 构成**精度阶对照**（同题库 odeProblems 复用）；真值=方法输出 y_N（后端用 rk4Column 现算不落库，精确解闭式记审计 ref_Y 不打分）；migration `018_add_rk4_ode.sql`（幂等）
2. **target 默认 exp_growth n=1 单步四阶**：K1..K4 = 1、1.5、1.75、2.75 全手算，终值恰 **65/24≈2.7083333**（经典教材习题）；y'=y 时每步恰乘稳定性函数 R(h)=1+h+h²/2+h³/6+h⁴/24（单测按 R(h)^n 恒等式交叉验证）
3. **防作弊语义（与 L20/L21 对照）**：RK4 收敛快，抄精确解 e 差仅 ~0.37% -> 约 72 分，能过线但非满分（**过程真伪由 y_col 审计暴露**，L19 龙贝格同款语义）；抄 L21 改进欧拉答案 2.640625 -> ~51 分、抄 L20 显式答案 2.44140625 -> ~30 分均不过线；stiff_decay 单步即发散（稳定性函数 **|R(−10)|=291>1**，终值恰 291），抄精确解 e^{−10} 或抄低阶方法发散值均 0 分
4. **readings/user_config 语义与 L20/L21 完全同款**：`{value, y_col?}`（y_col 长度 n+1 仅审计）；user_config 可换题/换区间/换步数 n=1~1024；题专属参数（a/b/n）归属规则同 v0.12.1
5. 11 个单测（`TestScoreRK4*`：闭式（65/24 / 1/3 / 291 / R(h)^n 恒等式）交叉验证/精确解（含 y_col 审计）/相对误差曲线/抄袭陷阱（抄 e=72 分、抄 L21/L20 不过线）/stiff 发散（换题带 n=1）/换题/换 n/换区间/y_col 审计/坏输入/target 参数归属）；前端新增 `runge-kutta-ode` 页（斜率场 + K1..K4 四探分步表）并注册；契约升 v0.17（§12 表 + 附录 L12 章节 + 链延 order 12，排队表仅剩 least_squares_fit）

## v0.18 变更摘要（2026-10-01）

1. **数值分析 L13 阿当姆斯预报-校正法上线**（experiment/level id 23，前置 L22，解锁链 order 13，难度 4，用户点名"微分方程数值解"方向）：`adams_pc_ode`，target.method=`adams_rel_err`；**线性多步法 AB4 外推预报 + AM4 内插校正一次（PECE 模式），表头 y_1..y_3 用 RK4 启动**（与 L22 同式，表头同阶精度），全局截断误差 O(h⁴)——与 L20 O(h)/L21 O(h²)/L22 O(h⁴) 构成**单步法/多步法方法族对照**（同题库 odeProblems 复用）；真值=方法输出 y_N（后端用 adamsPCColumn 现算不落库，精确解闭式记审计 ref_Y 不打分）；migration `019_add_adams_pc_ode.sql`（幂等）
2. **target 默认 exp_growth n=4**（3 步 RK4 表头 + 1 步 AB4-AM4，经典手算规模）：表头 y_1..y_3 = R(1/4)^i（R(h)=1+h+h²/2+h³/6+h⁴/24），终值 **2.7182259833**；单测闭式交叉验证（独立重算 AB4/AM4 公式 + 表头段与 rk4Column 同 h 逐点比对 + node 对拍锚点）
3. **防作弊语义（与 L20-L22 对照）**：四阶法收敛快，**抄精确解 e 约 93 分、抄 L22 默认答案 65/24 约 72 分**，均过线但非满分（同为四阶法数学上难分，过程真伪由 y_col 审计暴露，L19/L22 同款语义）；抄 L21 改进欧拉答案 2.640625 约 49 分、抄 L20 显式答案 2.44140625 约 30 分均不过线；stiff_decay（hλ=−2.5）**多步法绝对稳定区间远小于 RK4**，抄精确解 e^{−10} 直接 0 分、抄 RK4 n=4 表头答案 0.1768 约 9 分也被拦（多步法稳定性观察题）
4. **readings/user_config 语义与 L20-L22 同款，唯 n 下限抬高到 4**：`{value, y_col?}`（y_col 长度 n+1 仅审计）；user_config 可换题/换区间/换步数 **n=4~1024**（3 步表头+至少 1 步阿当姆斯，n<4 直接 400）；题专属参数（a/b/n）归属规则同 v0.12.1
5. 12 个单测（`TestScoreAdams*`：闭式交叉验证（R(1/4)^i 表头 + AB4/AM4 独立公式重算 + decay/stiff 锚点）/精确解（含 y_col 审计）/相对误差曲线/抄袭陷阱（抄 e=93、抄 L22=72、抄 L21/L20 不过线）/stiff 稳定性（正确递推 100 + 抄精确解 0 + 抄 RK4 表头不过线）/换题/换 n（n=8 锚点）/换区间/y_col 审计/坏输入（含 n=3 拦截）/target 参数归属）；前端新增 `adams-pc-ode` 页（斜率场 + RK4 表头 4 行 + AB4-AM4 预报校正分步表）并注册；契约升 v0.18（§12 表 + 附录 L13 章节 + 链延 order 13）

## v0.19 变更摘要（2026-10-01）

1. **数值分析 L14 最小二乘拟合上线**（experiment/level id 24，前置 L23，解锁链 order 14，难度 3，原排队表末位按序上线，**数值分析链收官**）：`least_squares_fit`，target.method=`lsq_rel_err`；**幂基法方程 Σ_k x_k^{i+j}·c_i = Σ_k x_k^j·y_k（j=0..m），列主元高斯求解（复用 L13 的 gaussSolve）**；与 L16/L18 插值"过所有点"相对照：LSQ 只使残差平方和最小、**不过点**；真值=法方程拟合多项式 P(x_eval)（后端 lsqFit 现算不落库，"真值=方法输出"同 L7/L9/L16/L20-L23）；migration `020_add_least_squares_fit.sql`（幂等）
2. **新增拟合题库 fitProblems（三题，前后端逐字同步）**：`lin_line` 直线拟合 4 点（法方程手算 **a=1.1、b=0.96，P(3.5)=4.46**）；`quad_parabola` 二次拟合 5 点（系数恰 **27/350、1/28、137/140，P(2)=711/175**，x_eval 落在数据点上）；`exp_lnw` 指数模型 y=a·e^{bx} 5 点（**ln 线性化**拟合，真值=e^{A+2.5B}=6.9103931）
3. **防作弊语义**：拟合不过点——抄观测值约 29/30/44 分、抄均值 16 分、exp 题不线性化直接拟 y 12 分均不过线；quad 抄真函数 x²=4 恰 58 分差线 2 分、次数用错（直线拟合 6.02）1 分；**抄三次插值 P3(3.5)=4.4375 约 69 分过线但非满分**（插值≠拟合，与 RK4 抄 e 同款"同为合法路径但不该得满分"语义，过程真伪由 coef 审计暴露）
4. **readings `{value 必填, coef? 可选审计}`**（系数长度须 m+1）；user_config 可换题/换拟合次数 **m=0~min(5, 点数-2)**（上界防拟合退化成插值——m=点数-1 时法方程解=插值多项式）/换求值点/自定义数据点 pts_x+pts_y（3~12 个互异，须带 x_eval）；m/x_eval 归属规则同 v0.12.1
5. 14 个单测（`TestScoreLSQ*`：闭式（手算法方程 1.1/0.96、系数分数 27/350、1/28、137/140、exp 2×2 闭式交叉验证）/正交投影不变量（残差与幂基正交 Σr·x^p=0，各题与换 m 全验）/精确解（含 coef 审计）/相对误差曲线/抄袭陷阱（抄观测/均值/插值）/quad 陷阱（抄观测 44、抄 x²=58、次数用错 1）/exp 线性化（正确 100 + 不线性化 12 + 观测 13/9）/换题/换 m（m=0 均值、m=2、quad m=1 直线语义满分、越界 400）/换 x_eval/自定义数据点（缺 x_eval、只给 pts_x、2 点、x 重复均 400）/coef 审计/坏输入/target 参数归属）；前端新增 `least-squares-fit` 页（散点 + 拟合曲线 + 求和表 + 法方程分步卡）并注册；契约升 v0.19（§12 表 + 附录 L14 章节 + 链延 order 14，**排队表清空**）

## v0.20 变更摘要（2026-10-03）

1. **数值分析 L15 三次样条插值上线**（experiment/level id 25，前置 L24，解锁链 order 15，**难度 4 与 L22/L23 并列最高**，用户点名方向插队，插值系列补最后经典）：`cubic_spline`，target.method=`spline_rel_err`；**自然边界三弯矩方程 mu_i·M_{i-1}+2M_i+lam_i·M_{i+1}=d_i（M_0=M_{n-1}=0），三对角追赶法（thomasSolve 新增）求解**；与 L16 拉格朗日单一高次多项式对照：分段低次治龙格现象；真值=样条 S(x_eval)（后端 splineMoments+splineEvalAt 现算不落库，"真值=方法输出"同 L7/L9/L16/L20-L24）；migration `021_add_cubic_spline.sql`（幂等）
2. **新增样条题库 splineProblems（三题，前后端逐字同步）**：`dist_4` 位移观测数据 4 节点不等距 h=1,1,2（三弯矩手算闭式 **M1=15/23、M2=78/23，S(3)=559/46**）；`sin_5` sin 五等距节点（**对称 M1=M3**，S(π/6)≈0.49974——光滑函数样条逼近极好）；`runge_5` 龙格函数五等距节点（**与 L16 同节点同求值点三法对照**：P(0.8)≈−0.379310 振荡 vs S(0.8)≈−0.036340 vs f=1/17；自然边界端段仍下弯，clamped 更准但需导数信息——教学点）
3. **防作弊语义**：dist_4 抄拉格朗日 P(3)=11.5 → **39 分**、抄线性内插 13 → **35 分**、抄观测全 0，只有真解三弯矩才过线；sin_5 **抄 sin(π/6)=0.5 → 83 分、抄拉格朗日 → 78 分过线但非满分**（过程真伪由 m_col 审计暴露，L19 抄 I / L22 抄 e 同款语义）、抄观测 ≤3 分；runge_5 抄 P/抄 f/抄观测**全 0 分**；自定义节点抄原函数（x² 节点抄 6.25）→ **58 分恰差线 2 分**（自然边界端段偏差）
4. **readings `{value 必填, m_col? 可选审计}`**（弯矩列长度须=节点数、含端点零弯矩）；user_config 可换题/换求值点/自定义节点 nodes_x+nodes_y（3~8 个且 **x 严格递增**，须带 x_eval 且**在节点区间内——样条不做外推**，与 L16 可外推的差异点）；x_eval 归属规则同 v0.12.1
5. 13 个单测（`TestScoreSpline*`：闭式（dist_4 手算 15/23、78/23、559/46 + 高斯独立解交叉验证、sin_5/runge_5 对称弯矩与 node 锚点、自定义 x² 6.35）/方程恒等式（三弯矩残差=0、节点复现 S(x_k)=y_k、自然边界、**追赶法 vs 稠密高斯双路径** 5 组全验）/精确解（含 m_col 与 ref_f 审计）/相对误差曲线/陷阱（dist_4 抄拉格朗日 39、线性内插 35、观测 0/sin_5 抄 0.5→83 过线不满分、抄 P→78、观测 ≤3/runge_5 抄全 0）/段端点求值（左端/中间/右端节点值满分）/换 x_eval（闭式 1471/368、超区间 400）/自定义节点（抄 6.25→58 差线 2 分；2 节点、9 节点、乱序、缺 x_eval、超区间、数量不匹配均 400）/m_col 审计/坏输入/target 参数归属）；前端新增 `cubic-spline` 页（散点 + 分段样条曲线 + 拉格朗日对照 + 三弯矩分步卡）并注册；契约升 v0.20（§12 表 + 附录 L15 章节 + 链延 order 15）

## v0.21 变更摘要（2026-10-09）

1. **数值分析 L16 隐式（后退）欧拉法上线**（experiment/level id 26，前置 L25，解锁链 order 16，难度 3，用户点名"微分方程数值解"再添一关）：`euler_implicit`，target.method=`euler_implicit_rel_err`；**y_{n+1}=y_n+h·f(x_{n+1},y_{n+1}) 隐式方程，题库线性齐次故闭式 y_{n+1}=y_n/(1−h·a(x_{n+1}))**；稳定性函数 R=1/(1−hλ) **A 稳定**，与 L20 显式欧拉圆盘 |1+z|≤1 互补——stiff_decay 上显式发散 5.0625 vs 隐式收敛 16/2401，绝对稳定性显隐对照主场；真值=方法输出 y_N（后端 eulerImplicit 现算不落库）；scoreEulerODE 重构为 eulerVariant 三态（explicit/improved/implicit）；migration `022_add_euler_implicit.sql`（幂等）
2. **防作弊语义（e2e 实测全中）**：exp_growth n=4 真值 (4/3)^4=256/81——抄精确解 e → 24、抄 L20 → 15、抄 L21 → 21、抄 L22 → 23、抄 L23 → 24 均不过线（一阶法截断误差 ~14% 全拦截）；stiff 抄精确解/抄显式发散值全 0；decay_sq 抄 e^{−1} → 36；**h·a(x)=1（exp_growth n=1）隐式方程无有限解 → 400**
3. 12 个单测 `TestScoreEulerImplicit*` 全绿（闭式 256/81 / (1−h)^{−n} 与 (1+10h)^{−n} A 稳定性恒等式 / 精确解+审计 / 曲线 / 五法陷阱 / stiff / 换题 / 换 n / 换区间 / 奇异步长 400 / target 归属）；前端新增 `euler-implicit` 页（斜率场 + **灰虚线 L20 显式对照列** + 隐式方程分步卡）并注册；契约升 v0.21（§12 表 + 附录 L16 章节 + 链延 order 16）；真库 e2e 全绿（16/16 avg 100）

## v0.22 变更摘要（2026-10-09）

1. **数值分析 L17 LU 分解（Doolittle）解线性方程组上线**（experiment/level id 27，前置 L26，解锁链 order 17，难度 3，用户点名"继续加实验"，AskUserQuestion 选定方向）：`lu_decomp`，target.method=`lu_rel_err`；**A=LU 三角分解不选主元（消去乘数 m_ik 存入 L、消去结果即 U）+ 前代 Ly=b + 回代 Ux=y**；与 L13 列主元高斯同族对照（消去过程数学相同，LU 乘数存下来可一次分解解多组 b；不选主元遇主元≈0 中断 → 400，正是稳定性差异教学点）；题库复用 linSystems 五题（均无需选主元）；真值=LU 路径解（后端 luDecompose 现算不落库，"真值=方法输出"同全系列）；migration `023_add_lu_decomp.sql`（幂等）
2. **防作弊语义**：解与 L13 列主元高斯数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 **l_col 乘子审计**暴露（L 的 l21,l31,l32,... 按行序，长度 n(n-1)/2，detail 记 l_col/ref_l 供教师对照）；抄 b 向量/零向量/分量换位低分不过线；默认题 sys3_basic 解恰 **(2,3,-1)**、乘子列闭式 **[-1.5,-1,4]** 全手算；**自定义矩阵主元≈0（[[0,1],[1,0]] 非奇异但首主元为 0）或奇异 → 400**
3. 10 个单测 `TestScoreLUDecomp*` 全绿（闭式（sys3_basic L/U/l_col 手算 + **库内五题与自定义阵 LU vs 列主元高斯同解恒等式**交叉验证）/精确解（含 l_col/ref_l 审计）/曲线（期望值用同一曲线 helper 计算钉接线）/陷阱（抄 b/零/换位不过线、乘子错解对仍 100）/高斯同解语义/换题 sys4_tridiag/自定义 2 阶/主元中断 400/坏输入 9 例/target 生效）；前端新增 `lu-decomp` 页并注册；契约升 v0.22（§12 表 + 附录 L17 章节 + 链延 order 17）

## v0.23 变更摘要（2026-10-09）

1. **数值分析 L18 追赶法解三对角方程组上线**（experiment/level id 28，前置 L27，解锁链 order 18，难度 2，用户点名"做追赶法"，L27 LU 分解的三对角特例）：`thomas_solve`，target.method=`thomas_rel_err`；**追**（Crout 消元）beta_1=b_1，m_i=a_{i-1}/beta_{i-1}，beta_i=b_i−m_i·c_{i-1}，d_i←d_i−m_i·d_{i-1}；**赶**（回代）x_n=d_n/beta_n，x_i=(d_i−c_i·x_{i+1})/beta_i——仅存三条对角线 O(n) 计算量（满阵 LU 为 O(n³/3)），大型三对角阵（热传导差分/样条三弯矩）的标准解法；与 L25 样条内部归一化版 thomasSolve 同族（本关 thomasChase 按教材原始形式留出 m/beta 审计）；真值=追赶法解（后端 thomasChase 现算不落库，"真值=方法输出"同全系列）；migration `024_add_thomas_solve.sql`（幂等）
2. **新增三对角题库 triSystems（紧凑 a/b/c/d 存储，四题均对角占优或对称正定——追赶法不选主元必安全）**：tri1_basic 手算例题 4 阶（m=(0.5,0.8,0.3125)，beta=(2,2.5,3.2,4.6875)，解恰 (1,2,3,4)）/ tri2_poisson 一维泊松差分 5 内点（beta=(2,3/2,4/3,5/4,6/5)，解全 1）/ tri3_small 入门 3 阶（解 (1,1,1)）/ tri4_spline 三弯矩系数阵 6 阶（L25 呼应，解全 1）
3. **防作弊语义**：解与 L13/L27 数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 **m/beta 双审计**暴露（m 长度 n-1、beta 长度 n，各全或无，detail 记 m/ref_m、beta/ref_beta 供教师对照）；抄 d 向量/零向量/分量换位低分不过线；**自定义三对角 beta≈0（b1=0 非奇异需选主元、或中途 beta2=0）→ 400**——追赶法适用条件（对角占优/对称正定）的教学点
4. 9 个单测 `TestScoreThomas*` 全绿（闭式（tri1_basic m/beta/解 + tri2_poisson beta 通项 + **题库四题与自定义阵 追赶 vs 列主元高斯 vs LU 三路恒等**交叉验证）/精确解（含 m/beta 双审计）/曲线（期望值用同一曲线 helper 计算钉接线）/陷阱（抄 d/零/换位不过线、m 错解对仍 100）/高斯同解语义/换题 tri2_poisson/自定义 3 阶/beta 中断 400/坏输入 10 例 + 9 阶超限/target 生效）；前端新增 `thomas-solve` 页（三对角带状矩阵 + 追赶工作表逐行演化 + 解向量条形图逐根长出）并注册；契约升 v0.23（§12 表 + 附录 L18 章节 + 链延 order 18）

## v0.24 变更摘要（2026-10-11）

1. **数值分析 L19 平方根法（Cholesky）解对称正定方程组上线**（experiment/level id 29，前置 L28，解锁链 order 19，难度 2，用户点名"继续补充新的实验"，直接法一章收官）：`cholesky_solve`，target.method=`cholesky_rel_err`；**A = L·Lᵀ** 按行 Cholesky–Crout 分解——l_ii=sqrt(a_ii−Σ_{k<i} l_ik²)，l_ij=(a_ij−Σ_{k<j} l_ik·l_jk)/l_jj（j<i，L 主对角全正），再前代 Ly=b + 回代 Lᵀx=y；对称正定保证根号内恒正、永不选主元，计算量约为 LU 的一半（n³/6 vs n³/3）——协方差矩阵/有限元刚度阵的标准解法；真值=平方根法解（后端 choleskySolve 现算不落库，"真值=方法输出"同全系列）；migration `025_add_cholesky_solve.sql`（幂等）
2. **新增对称正定题库 spdSystems（四题，L/y/x 全整数，课堂手算友好）**：spd1_2x2 入门 2 阶（L=[[2,0],[1,2]]，y=(3,2)，解恰 (1,1)）/ spd2_min3 min(i,j) 阵 3 阶默认题（L=下三角全 1，y=(6,5,3)，解恰 (1,2,3)）/ spd3_tridiag3 三对角 SPD（L28 呼应，y=(3,3,3)，解恰 (1,1,1)）/ spd4_min4 min(i,j) 阵 4 阶（y=(4,3,2,1)，解全 1）
3. **防作弊语义**：解与 L13/L27/L28 数学同源（同 L18/L16 先例：**抄=算对**），过程真伪由 **L/y 双审计**暴露（L 取下三角含主对角 n(n+1)/2 个元、y 长度 n，各全或无，detail 记 L/ref_L、y/ref_y 供教师对照）；抄 b 向量/零向量/分量换位低分不过线；**自定义 A 非对称或根号内≤0（不定阵/半正定奇异阵）→ 400**——平方根法适用条件（对称正定）的教学点，与 L13 列主元高斯的差异
4. 10 个单测 `TestScoreCholesky*` 全绿（闭式（四题 L/y/x 手算 + **题库四题与自定义阵 LLᵀ 重建恒等式 + 平方根法 vs 列主元高斯 vs LU 三路同解**交叉验证）/精确解（含 L/y 双审计）/曲线（期望值用同一曲线 helper 计算钉接线）/陷阱（抄 b/零/换位不过线、L/y 错解对仍 100）/高斯同解语义/换题 spd4_min4/自定义 2 阶/非正定 400（不定+半正定+非对称三例）/坏输入 10 例 + 6 阶超限/target 生效）；前端新增 `cholesky-solve` 页并注册；契约升 v0.24（§12 表 + 附录 L19 章节 + 链延 order 19）
