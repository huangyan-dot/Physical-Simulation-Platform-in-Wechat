# backend - 大学物理实验 3D 模拟小程序后端

阶段 0 骨架 + 阶段 1 全量接线 + M2 中间件 + 契约 v0.4 补齐 + v0.5 滑块修复+审计补全 + v0.6-v0.9 数值分析实验批次 + v0.10 物理实验批次下线 + v0.11 SOR 迭代上线：Gin + GORM + MySQL + viper + zap，已验证全链路（HTTP -> 路由 -> middleware -> controller -> service -> repository -> MySQL -> JSON）。

文档：`docs/api-contract.md`（接口契约 v0.11，改动需全组知晓）、`docs/database-design.md`（数据库设计）。

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
