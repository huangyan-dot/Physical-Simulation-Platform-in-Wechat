# 接口契约（API Contract）v0.11

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
    "status": "locked", "difficulty": 2, "best_score": 0 }
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

**编号约定（v0.10 起）**：数值分析实验 id 从 11 起（bisection=11、newton_iter=12、gauss_elim=13、jacobi_iter=14、sor_iter=15）；物理实验 id 1-2（牛顿环/示波器）已随 v0.10 下线，1-10 空置保留。**解锁链（v0.11）**：L11 二分法(order 1，链头无前置) -> L12 牛顿迭代(order 2) -> L13 高斯消去(order 3) -> L14 雅可比迭代(order 4) -> L15 SOR(order 5)。

### 实验清单与进度

| level_id | exp_id | code | 实验名 | 难度 | 状态 |
|---|---|---|---|---|---|
| 11 | 11 | `bisection` | 二分法求根 | 1 | ✅ v0.6 上线 |
| 12 | 12 | `newton_iter` | 牛顿迭代法求根 | 2 | ✅ v0.6 上线 |
| 13 | 13 | `gauss_elim` | 高斯消去法解线性方程组 | 2 | ✅ v0.8 上线（插队） |
| 14 | 14 | `jacobi_iter` | 雅可比迭代法解线性方程组 | 2 | ✅ v0.9 上线（插队） |
| 15 | 15 | `sor_iter` | SOR 逐次超松弛迭代法解线性方程组 | 2 | ✅ v0.11 上线（插队） |
| - | - | `lagrange_interp` | 拉格朗日插值 | 2 | 排队 |
| - | - | `simpson_integration` | 复合辛普森数值积分 | 3 | 排队 |
| - | - | `least_squares_fit` | 最小二乘拟合 | 3 | 排队 |
| - | - | `runge_kutta_ode` | 龙格-库塔法解 ODE | 4 | 排队 |

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

## 附：物理实验批次（v0.7 恢复，v0.10 下线）

牛顿环（level 1）与示波器（level 2）曾按《开发任务讲解》PDF 恢复为交付实验（v0.7，2026-08-25）。v0.10（2026-09-09）随项目聚焦数值分析实验链而**整体下线**：migration `011_remove_ring_oscilloscope.sql` 删除 experiments/levels/user_progresses/operation_logs 对应行并重排解锁链；`scoring.go` 评分器与 `scorers` 注册、前端 app.json/index.js 页面注册同步移除（前端页面文件按项目惯例留在磁盘不打包）。历史接口格式（newton_ring 的 least_squares_R 拟合评分、oscilloscope 的 param_match 通道评分）见契约 v0.7-v0.9 归档版本。
