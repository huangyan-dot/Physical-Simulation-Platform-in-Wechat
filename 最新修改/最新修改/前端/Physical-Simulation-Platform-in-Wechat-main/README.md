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
│       └── oscilloscope/     # 示波器实验（2D canvas 动画）
```

## 快速开始

1. 打开微信开发者工具，选择「导入项目」。
2. 选择本 `miniprogram` 目录。
3. 修改 `app.js` 中的 `apiBaseUrl` 为你的后端地址（默认 `http://localhost:8080`）。
4. 修改 `project.config.json` 中的 `appid` 为你申请的微信小程序 AppID。
5. 点击「编译」即可预览。

## 后端接口约定

前端默认对接以下接口（与开发文档一致）：

- `POST /auth/login` - 微信登录
- `GET /auth/me` - 获取当前用户信息
- `PUT /users/:id` - 补全用户信息
- `GET /classes` - 班级列表
- `POST /classes` - 创建班级
- `DELETE /classes/:id` - 删除班级
- `POST /classes/:id/members` - 添加班级成员
- `GET /levels` - 关卡列表
- `GET /experiments/:id` - 实验配置
- `GET /progress/mine` - 我的进度
- `GET /progress/class/:id` - 班级成绩
- `POST /progress/submit` - 提交实验读数

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
- 开发阶段后端未就绪时，可在 `request.js` 中配置 Mock 数据或关闭网络校验。
- 登录依赖微信小程序 AppID 和 Secret，请确保后端 `/auth/login` 接口正确实现 `jscode2session`。
