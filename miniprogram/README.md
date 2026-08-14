# 大学生物理实验 3D 模拟 - 微信小程序前端

## 项目说明

本项目是基于微信原生小程序框架（WXML/WXSS/JS）开发的大学生物理实验 3D 模拟应用前端。可直接导入微信开发者工具运行。

## 目录结构

```
miniprogram/
├── app.js                    # 小程序入口，登录态管理
├── app.json                  # 全局页面与 tabBar 配置
├── app.wxss                  # 全局样式
├── project.config.json       # 开发者工具项目配置
├── sitemap.json              # 站点索引
├── custom-tab-bar/           # 自定义底部 tabBar
├── utils/                    # 工具函数
│   ├── config.js             # 后端地址等全局配置（改这里即可切换环境）
│   ├── request.js            # HTTP 请求封装（自动带 token）
│   ├── storage.js            # 本地存储封装
│   └── auth.js               # 登录/权限相关
├── engine/                   # 3D 引擎封装（预留 three.js）
│   └── three-core.js
├── pages/
│   ├── index/                # 首页：实验关卡列表
│   ├── login/                # 登录/补全学籍信息
│   ├── class/                # 班级管理
│   ├── profile/              # 个人中心
│   ├── progress/             # 成绩进度（学生/教师双视角）
│   └── experiments/
│       ├── newton-ring/      # 牛顿环实验（2D canvas）
│       ├── oscilloscope/     # 示波器实验（2D canvas 动画）
│       └── pendulum/         # 单摆实验（2D canvas 动画）
└── 后端代码/                 # Go + Gin + MySQL 后端（独立服务）
```

## 快速开始

### 1. 启动后端

双击项目根目录的 `start_backend.bat`（会依次启动 MySQL、建库建用户、跑 Go 服务）。

启动成功的标志：控制台出现 `MySQL 已连接` 和 `server listening {"addr": ":8080"}`。

自检：浏览器访问 <http://localhost:8080/ping>，应返回 `{"message":"pong"}`。

> 若看到 `MySQL 连接失败，仅 /ping 可用`，说明数据库没连上，此时所有业务接口都不会注册（小程序会报「资源不存在」）。请检查 MySQL 是否在 3306 端口运行。

### 2. 启动小程序

1. 打开微信开发者工具，选择「导入项目」，目录选本 `miniprogram` 文件夹。
2. 若用**模拟器**调试：`utils/config.js` 里的 `localhost:8080` 无需修改。
3. 若用**真机预览**：必须把 `utils/config.js` 的 `API_BASE_URL` 改成电脑局域网 IP，
   例如 `http://192.168.1.100:8080`（`ipconfig` 查看 IPv4 地址）。
   手机需与电脑同一 WiFi，并允许防火墙放通 8080 端口。
   > 真机上 `localhost` 指手机自己，不是你的电脑，因此一定连不上。
4. 开发期需在「详情 → 本地设置」勾选**不校验合法域名**（本项目 `project.private.config.json` 已设 `urlCheck: false`）。
5. 点击「编译」。首页点「开发登录」即可用后端内置测试账号（`dev_student`）登录。

### 3. 测试账号（开发后门）

后端 `configs/config.yaml` 中 `allow_dev_backdoor: true` 时可用，登录 `code` 直接传：

| code | 角色 | 用途 |
|------|------|------|
| `dev_student` | student | 做实验、提交读数、看个人成绩 |
| `dev_teacher` | teacher | 建班级、加成员、看班级看板 |
| `dev_admin` | admin | 查看操作审计日志 |

**上线前必须**：把 `allow_dev_backdoor` 设为 `false`，并填好 `wechat.appid` / `wechat.secret`。

## 后端接口约定

前端对接以下接口（详见 `后端代码/docs/api-contract.md`）：

- `POST /auth/login` - 微信登录（`code` 传 `dev_*` 走开发后门）
- `GET /auth/me` - 获取当前用户信息
- `PUT /users/:id` - 补全用户信息
- `GET /classes` - 班级列表
- `POST /classes` - 创建班级（teacher/admin）
- `DELETE /classes/:id` - 删除班级
- `POST /classes/:id/members` - 添加班级成员
- `GET /levels` - 关卡列表（带个性化 status / best_score）
- `GET /experiments/:id` - 实验配置（**:id 传 level_id**）
- `GET /progress/mine` - 我的进度
- `GET /progress/class/:classId` - 班级成绩（teacher/admin）
- `POST /progress/submit` - 提交实验读数（student，限流 20 次/分钟）
- `GET /admin/operation-logs` - 操作审计日志（admin）

### 响应约定（重要）

成功响应**直接返回数据本体**，不包 `{code:0,data:...}`。因此页面里直接用
`const data = await request(...)`，**不要**再写 `res.data`。
失败由 HTTP 状态码表达，`request.js` 统一转成 reject，401 会自动清登录态并跳登录页。

## 数据说明

前端不再内置 mock 假数据：接口失败时页面显示错误提示和「重试」按钮，
而不是静默展示假数据（避免后端故障被伪装成正常状态）。
关卡、实验参数、班级等初始数据由后端启动时自动 seed 写入 MySQL。

## 启用 3D（three.js-miniprogram）

当前实验页面使用 Canvas 2D 实现核心可视化逻辑，保证无 npm 依赖即可运行。

如需接入 three.js 3D 场景：

1. 在项目根目录执行：
   ```bash
   npm init -y
   npm install three.js-miniprogram
   ```
2. 在微信开发者工具中点击「工具 -> 构建 npm」。
3. 取消 `engine/three-core.js` 中的注释，按 three.js-miniprogram 文档接入 WebGL。
4. 在实验页面将 `<canvas type="2d">` 改为 `<canvas type="webgl">` 并挂载 three.js 渲染器。

## 注意事项

- 小程序主包体积限制 2MB，three.js 较大，建议放入分包按需加载。
- 登录依赖微信小程序 AppID 和 Secret，上线时后端需正确实现 `jscode2session`。

## 常见问题

| 现象 | 原因 / 解决 |
|------|------------|
| 页面显示「网络请求失败」 | 后端没启动，或真机用了 `localhost`。改 `utils/config.js` 为电脑局域网 IP |
| 所有接口返回「资源不存在」 | 后端 MySQL 没连上，业务路由未注册。检查 MySQL 是否运行 |
| 提示「登录已过期」并跳登录页 | token 失效（默认 72 小时），重新登录即可 |
| 提交读数返回「操作过于频繁」 | 触发限流（20 次/分钟），稍等再试 |
| 关卡显示「🔒 未解锁」 | 需先通过前置关卡；第 1 关默认解锁 |
| 得分为 0 | 读数与理论值偏差过大。误差约 8% 得 60 分，误差 20% 即 0 分 |
