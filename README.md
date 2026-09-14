# 公司员工收益管理系统 (OPS Admin)

员工、账号、收益、提现、审核、结算与审计一体化后台。

## 技术栈

- 前端：React 18 + TypeScript + Vite + Ant Design
- 后端：Go 1.23
- 数据库：PostgreSQL（生产使用 Neon）
- 前端托管：Vercel
- 后端托管：Render

## 本地开发

### 后端

设置 PostgreSQL 连接串：

```bash
export DATABASE_URL='postgresql://...'
go run ./cmd/server -demo=true -addr 127.0.0.1:8000
```

服务启动时会自动读取 `backend/schema.sql` 初始化缺失表；数据库为空且 `-demo=true` 时会写入合成演示数据。

### 前端

```bash
cd frontend
npm install
npm run dev
```

Vite 开发服务器会把 `/api` 代理到 `http://127.0.0.1:8000`。

生产构建：

```bash
cd frontend
npm run build
```

构建结果写入根目录 `web/`。

## 演示账号

仅用于演示环境：

- 管理员：`admin / DemoAdmin2026!`
- 运营：`operator / DemoOperator2026!`
- 财务：`finance / DemoFinance2026!`
- 只读：`viewer / DemoViewer2026!`

正式使用前请创建自己的管理员并停用演示账号。

## 主要功能

- 登录 / HttpOnly Cookie 会话 / CSRF
- admin / operator / finance / viewer 权限
- 运营 Dashboard 与监控
- 员工、小组、实名人员、实名渠道
- 聊天账号、平台与应用配置
- 收益明细与收益报表
- 提现申请、财务审核、驳回、支付登记
- 实名人员对账、结算账单
- 黑名单、系统用户、操作日志
- 提现与结算幂等保护
- 收益修正采用追加流水，保留审计轨迹

本系统只登记外部支付凭证，不直接执行真实资金转账。

## 目录

```text
cmd/server/          Go 服务入口
internal/app/        API、鉴权、业务逻辑、PostgreSQL 适配
backend/schema.sql   PostgreSQL 数据库结构
frontend/            React + TypeScript + Vite 源码
web/                 已构建静态前端资源
docs/                测试与验收记录
render.yaml          Render 部署配置
DEPLOYMENT.md        生产部署说明
```

## 生产架构

```text
Browser
   |
Vercel (React / same-origin /api)
   |
Render (Go API)
   |
Neon PostgreSQL
```

浏览器始终请求 Vercel 域名下的 `/api/*`，再由 Vercel rewrite 转发到 Render，从而维持 Cookie、CSRF 和同源写保护。
