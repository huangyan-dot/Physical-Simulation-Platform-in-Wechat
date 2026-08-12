# 接口契约（API Contract）v0.5

> 前后端并行开发的"合同"（对应设计文档第七节、10.4 约束一）。
> **本版已与前端仓库 `Physical-Simulation-Platform-in-Wechat-main` 的实际调用逐一对齐。**
> **v0.3：全部 12 个接口已在 `router.go`+`main.go` 装配接线，可端到端联调。**
> **v0.4（2026-08-10）：对照任务书补齐 5 个接口——§14 班级详情 / §15 班级改名 / §16 移除成员（M2 系列 RESTful 补全）、§17 实验列表（M3）、§18 学生查自己审计日志（M6 的 `/audit/mine`）。已全量接线+真库实测。**
> **v0.5（2026-08-12）：① §12 提交接口新增可选 `user_config` 字段，解决"滑块改仿真真值但评分 target 固定"问题；② 审计日志补全--登录、班级 CRUD、成员增删均写 `operation_logs`（M6 验收要求"登录/建班有日志"）。**
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
  { "id": 1, "name": "牛顿环实验", "experiment_code": "newton_ring",
    "status": "passed", "difficulty": 2, "best_score": 88 },
  { "id": 2, "name": "示波器实验", "experiment_code": "oscilloscope",
    "status": "unlocked", "difficulty": 3, "best_score": 0 },
  { "id": 3, "name": "单摆实验", "experiment_code": "pendulum",
    "status": "locked", "difficulty": 1, "best_score": 0 }
]
```

- `status`：`locked` / `unlocked` / `in_progress` / `passed`
- 解锁规则：第 1 关默认 unlocked；其余关前置关 passed 后解锁

## 9. GET /experiments/:id

鉴权：JWT。**:id 是 level_id**（见 0.1-2）。

**响应 200**

```json
{
  "id": 1,
  "code": "newton_ring",
  "name": "牛顿环实验",
  "render_mode": "mixed_3d_2d",
  "config": {
    "wavelength_nm": 589.3,
    "lens_radius_mm": 855,
    "k_range": [1, 10],
    "tolerance_mm": 0.02
  },
  "target": { "method": "least_squares_R" }
}
```

- `config` 结构随实验类型不同（MySQL JSON 字段）

## 10. GET /progress/mine

鉴权：JWT（学生看自己）。

**响应 200**

```json
{
  "total": 3,
  "passed": 1,
  "avg_score": 88,
  "best_score": 88,
  "records": [
    { "id": 1, "level_name": "牛顿环实验", "status": "passed", "score": 88, "best_score": 88, "attempts": 3 },
    { "id": 2, "level_name": "示波器实验", "status": "in_progress", "score": 0, "best_score": 0, "attempts": 1 }
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
  "level_id": 1,
  "experiment": "newton_ring",
  "readings": [ { "k": 1, "r": 0.42 }, { "k": 2, "r": 0.61 }, { "k": 3, "r": 0.75 } ],
  "user_config": { "lens_radius_mm": 855, "wavelength_nm": 589.3 }
}
```

**`user_config`（v0.5 新增，可选）**

前端实验页的滑块可能改变仿真真值（牛顿环 `lens_radius_mm`/`wavelength_nm`、单摆 `gravity`/`length_m`）。
若提交时携带 `user_config`，后端用它覆盖 DB target 中对应的真值后再评分，确保"学生调滑块后认真测量仍能得高分"。
不携带时回退到 DB 固定 target（向后兼容）。

| 实验 | user_config 可覆盖字段 |
|------|----------------------|
| newton_ring | `lens_radius_mm`, `wavelength_nm` |
| pendulum | `gravity`, `length_m` |
| oscilloscope | 无（无滑块问题） |

**响应 200**

```json
{ "score": 88, "passed": true, "best_score": 90, "unlocked_level_id": 2 }
```

- 后端按实验公式算分；`best_score` 取历史最佳（防重复刷分）
- 每次提交写 `operation_logs` 审计表
- **限流**：每用户每分钟 20 次（`config.rate_limit.submit_per_minute`），超限 429

**错误**：400（readings 非法，含空数组）、403（非学生）、429（刷分限流）

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
    { "id": 9, "user_id": 2, "action": "submit", "level_id": 1, "score": 100, "detail": "R̂=855.00mm...", "created_at": "2026-08-01T..." }
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
  { "id": 1, "code": "newton_ring", "name": "牛顿环实验", "render_mode": "mixed_3d_2d" },
  { "id": 2, "code": "oscilloscope", "name": "示波器实验", "render_mode": "mixed_3d_2d" }
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
