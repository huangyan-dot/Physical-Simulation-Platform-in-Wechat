# backend - 大学物理实验 3D 模拟小程序后端

阶段 0 骨架 + 阶段 1 全量接线 + M2 中间件 + 契约 v0.4 补齐 + v0.5 滑块修复+审计补全：Gin + GORM + MySQL + viper + zap，已验证全链路（HTTP -> 路由 -> middleware -> controller -> service -> repository -> MySQL -> JSON）。

文档：`docs/api-contract.md`（接口契约 v0.5，改动需全组知晓）、`docs/database-design.md`（数据库设计）。

## 目录结构

```
cmd/server/main.go     # 入口：加载配置 -> 连库 -> AutoMigrate 7 表 -> seed -> 装配依赖 -> 起服务
cmd/server/seed.go     # 启动期幂等种子数据：dev 账号 / 3 实验 / 3 关卡 / 示范班级
internal/
  config/              # viper 读 configs/config.yaml
  router/              # Gin 路由注册（全部 12 接口 + 鉴权 + 角色守卫）
  middleware/          # JWT 鉴权 / RequireRole 角色守卫
  controller/          # HTTP 入参出参，不碰 SQL
  service/             # 业务逻辑（含 scoring.go 三实验评分）
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

服务启动时自动 `AutoMigrate` 全部 7 张表，并幂等写入种子数据（dev 账号、3 个实验、3 个关卡、示范班级）。MySQL 不可用时降级为仅 `/ping` 可用。

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
