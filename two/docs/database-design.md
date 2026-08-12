# 数据库设计文档

对应《开发任务讲解》第八节（数据库设计）。当前实现 7 张表 + 2 个只读视图，库名 `physics_lab`，业务账号 `physics`。

## 1. ER 简图

```
users ──< class_members >── classes
  │                          │
  │<──── user_progress >──── levels >── experiments
  │
  │<──── operation_logs
```

## 2. 表结构

### 2.1 users（用户）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AI | 主键 |
| openid | VARCHAR(64) UNIQUE NOT NULL | 微信唯一身份；dev 账号为 `oDEV_STUDENT`/`oDEV_TEACHER`/`oDEV_ADMIN` |
| role | VARCHAR(16) DEFAULT 'student' | student / teacher / admin，带索引 |
| name | VARCHAR(64) NULL | 姓名；NULL 表示首次登录需补全 |
| student_no | VARCHAR(32) NULL | 学号 |
| created_at / updated_at | DATETIME | |

GORM model: `internal/model/user.go`

### 2.2 classes（班级）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AI | |
| name | VARCHAR(64) | 班级名 |
| teacher_id | BIGINT | 归属教师（users.id） |
| created_at / updated_at | DATETIME | |

### 2.3 class_members（班级成员）

| 字段 | 类型 | 说明 |
|------|------|------|
| class_id | BIGINT | 联合唯一键之一 |
| user_id | BIGINT | 联合唯一键之一（一个学生同班只一条） |
| joined_at | DATETIME | |

唯一约束 `uk_class_user(class_id, user_id)` 防重复加人。

### 2.4 experiments（实验定义）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AI | |
| code | VARCHAR(32) UNIQUE | newton_ring / oscilloscope / pendulum |
| name | VARCHAR(64) | |
| render_mode | VARCHAR(32) | 2d / iso_2_5d / perspective_3d / mixed_3d_2d |
| config | JSON | 实验参数（波长、透镜半径、g 真值等），随实验类型不同 |
| target | JSON | 评分目标；**必含 `pass_score`**（scoring.go 用 `score>=pass_score` 判过关，缺失则恒过关） |

种子数据见 `cmd/server/seed.go`。注意：oscilloscope 的 config.channels 键必须大写 `CH1`/`CH2`（与前端提交大小写一致）。

### 2.5 levels（关卡）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AI | |
| experiment_id | BIGINT | 所属实验 |
| name | VARCHAR(64) | |
| `order` | INT | 同实验内排序；解锁规则 = 前一关 passed |
| unlock_prev | TINYINT(1) DEFAULT 1 | 是否需要前关通过才解锁 |
| created_at / updated_at | DATETIME | |

### 2.6 user_progress（成绩与进度）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AI | |
| user_id | BIGINT | 联合唯一键之一 |
| level_id | BIGINT | 联合唯一键之一（一人一关一条） |
| best_score | INT | 历史最佳（防刷分以此为准） |
| last_score | INT | 最近一次得分 |
| attempts | INT | 尝试次数 |
| passed | TINYINT(1) | 是否已过关 |
| created_at / updated_at | DATETIME | |

唯一约束 `uk_user_level(user_id, level_id)`。

### 2.7 operation_logs（审计日志）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AI | |
| user_id | BIGINT | 操作者 |
| action | VARCHAR(32) | login / submit / class.create / class.delete / class.rename / class.member.add / class.member.remove |
| level_id | BIGINT NULL | 关联关卡（submit 时有值） |
| score | INT NULL | 得分（submit 时有值） |
| detail | VARCHAR(255) | 评分摘要 / 操作摘要 |
| created_at | DATETIME | |

索引：`idx_user(user_id)`、`idx_action(action)`、`idx_level(level_id)`、`idx_user_time(user_id, created_at)`、`idx_action_time(action, created_at)`。
复合索引服务于按"用户+时间"和"动作+时间"检索审计日志的高频场景（对应文档 8.3 设计）。

## 3. 只读视图（GORM 建的是表，查询走 Table() 指定）

| 视图/表 | 用途 | 消费接口 |
|---------|------|----------|
| class_summaries | 班级列表：member_count + 视角色计算的 my_pass_count（学生看已过/总数、教师看成员总数） | GET /classes |
| class_progress_rows | 班级成绩行：按学生 × 关卡聚合 best/last/attempts/passed | GET /progress/class/:id |

## 4. 初始化与迁移

- `migrations/001_init.sql`：建库 + 业务账号授权
- `migrations/002_core_tables.sql`：7 表 + 种子（与 seed.go 等价的手工版）
- 运行时：服务启动 `AutoMigrate` 全部表 + `cmd/server/seed.go` 幂等种子（dev 账号 ×3、实验 ×3、关卡 ×3、示范班级 + 学生入班）。MySQL 不可用时降级为仅 `/ping`。

## 5. 运维注意

- Windows mysql 客户端执行含中文 SQL 必须加 `--default-character-set=utf8mb4`，否则报 ERROR 1366
- 中文请求体请用 UTF-8 工具（Apifox）发送；Git Bash 的 curl 直接写中文会按本地代码页编码导致乱码入库
- 上线前：`config.allow_dev_backdoor=false`、`mode=release`、CORS 白名单收紧
