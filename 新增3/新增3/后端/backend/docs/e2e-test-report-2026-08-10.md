# 端到端联调报告（2026-08-10）

环境：真 MySQL（MySQL97 / physics_lab）+ `go run` 构建的 server.exe :8080，curl 直测。
结论：**13 接口全链路通过**，学生/教师/admin 三线 + 权限守卫 + 限流 + 审计均符合契约 v0.3。

## 学生线

| # | 用例 | 结果 |
|---|------|------|
| 1 | `POST /auth/login` dev_student | ✅ 签发 token + user（need_complete=false） |
| 2 | `GET /auth/me` | ✅ |
| 3 | `GET /levels` | ✅ 3 关，status/best_score 个性化正确 |
| 4 | `GET /experiments/1` | ✅ config+target 下发（target 含 pass_score） |
| 5 | submit 牛顿环准确读数 | ✅ score=100, passed, unlocked_level_id=2 |
| 6 | submit 牛顿环离谱读数 | ✅ score=0, passed=false，best_score 保持 100 |
| 7 | submit 空 readings | ✅ 400 |
| 8 | submit 示波器（CH2 微偏 f=51/A=1.4） | ✅ score=89（平均相对误差 2.17%） |
| 9 | submit 单摆（calc_g=9.79） | ✅ score=99 |
| 10 | `GET /progress/mine` | ✅ total/passed/avg/best/records 聚合正确 |

## 教师 / admin 线

| # | 用例 | 结果 |
|---|------|------|
| 11 | 教师 `GET /classes` | ✅ 含 teacher_name/member_count |
| 12 | 教师 `GET /progress/class/1` | ✅ summary（avg_score/pass_rate）+ rows |
| 13 | 学生访问班级成绩 | ✅ 403 |
| 14 | 学生尝试建班 | ✅ 403 |
| 15 | admin `GET /admin/operation-logs?action=submit` | ✅ 审计含评分 detail + 原始 readings |
| 16 | 教师访问审计日志 | ✅ 403 |
| 17 | 无 token 访问 /levels | ✅ 401 |
| 18 | 学生连发 22 次 submit | ✅ 前 20 次 200，第 21 起 429 |

注：审计分页参数名是 `page`/`size`（契约 §admin），不是 `page_size`。

## 前端字段核对（静态走查）

- 三实验页 submit 的 readings 键与 scoring.go 完全一致：`{k,r}` / `{channel:'CH1',f,A}` / `{period,calc_g}`；CH1/CH2 大写一致 ✅
- `PUT /users/:id` 载荷 `{name, student_no, class_id}` 与 auth.go CompleteProfile 一致（class_id>0 同时入班）✅
- 前端统一 `res.data || res` 兼容裸响应，与后端"不包 {code:0}" 约定兼容 ✅
- apiBaseUrl 已指向 LAN IP `http://192.168.31.16:8080`（真机联调需确认本机 IP 未变）

## ⚠️ 待全组确认：滑块改真值 vs 固定 target

- ~~单摆页可调 `gravity` 滑块、牛顿环页可调 `lens_radius_mm` 滑块，**会改变前端仿真的真值**；~~
- ~~但评分 target 固定为 DB 种子值（g=9.8、R=855mm）。~~
- ~~后果：学生改动滑块后认真测量反而得 0 分，永不通过。~~

**✅ v0.5 已修复（2026-08-12）**：采用方案②--提交时携带 `user_config`，后端按实际参数动态生成 target。
- 牛顿环 submit 带 `{user_config: {lens_radius_mm, wavelength_nm}}`
- 单摆 submit 带 `{user_config: {gravity, length_m}}`
- 示波器无滑块问题，不需要
- 不携带 `user_config` 时回退到 DB 固定 target（向后兼容）
- 新增 2 个单测验证：`TestScoreByExperiment_NewtonRingUserConfigOverridesTarget` / `TestScoreByExperiment_PendulumUserConfigOverridesTarget`

## ⚠️ 审计日志不完整

- ~~`operation_logs` 只记录了 `submit` 动作，但 PDF 要求"登录/建班都有日志"（M6验收标准）。~~

**✅ v0.5 已修复（2026-08-12）**：
- 登录写 `action=login` 审计日志
- 班级创建/删除/改名/加成员/移成员分别写 `class.create`/`class.delete`/`class.rename`/`class.member.add`/`class.member.remove`
- 审计日志写入失败不阻断业务（`_ = err`）
- 所有单元/集成测试已更新并通过

## 下一步

1. 真机联调：填 config.yaml 的 wechat.appid/secret；确认 LAN IP。
2. 上线前：`allow_dev_backdoor=false`、`mode=release`、CORS 白名单收紧、JWT secret 更换。
3. ~~滑块问题定案后回归一次三实验评分。~~ ✅ v0.5 已修复，前端已带 user_config 提交。
