# OPS Admin — React + Go + PostgreSQL

员工收益管理后台，前端使用 React 18 + TypeScript + Vite + Ant Design 5，后端使用 Go 1.23 HTTP API，生产数据库使用 PostgreSQL（Neon）。

## 技术栈

- 前端源码：`frontend/`
- 后端入口：`cmd/server/`
- 业务/API：`internal/app/`
- 数据库结构：`backend/schema.sql`
- 生产静态资源：`web/`

## 本地开发

后端需要提供 `DATABASE_URL`：

```bash
DATABASE_URL='postgresql://...' go run ./cmd/server
```

前端：

```bash
cd frontend
npm ci
npm run dev
```

Vite 开发服务器会把 `/api` 代理到 `http://127.0.0.1:8000`。

生产构建：

```bash
cd frontend
npm ci
npm run typecheck
npm run build
```

构建结果写入根目录 `web/`。

## 云端部署

- Go API：Render
- PostgreSQL：Neon
- React 前端：Vercel
- Vercel `/api/*` 通过 `vercel.json` 同源转发到 Render API，因此浏览器仍使用同源 Cookie / CSRF 模型。

Render 运行时至少需要：

- `DATABASE_URL`
- `OPS_DEMO=true`（仅演示环境）
- `OPS_COOKIE_SECURE=true`

演示账号：

- 管理员：`admin / DemoAdmin2026!`
- 运营：`operator / DemoOperator2026!`
- 财务：`finance / DemoFinance2026!`
- 只读：`viewer / DemoViewer2026!`

## 已实现

- 登录 / 会话鉴权
- 运营 Dashboard 与监控
- 员工、小组、实名人员、实名渠道
- 聊天账号
- 收益明细与收益报表
- 提现申请、财务审核、驳回、支付登记
- 实名人员对账、结算账单
- 应用配置、黑名单、运营渠道、平台
- 系统用户与操作日志

## 权限与账务规则

- HttpOnly Cookie 会话 + CSRF
- admin / operator / finance / viewer 权限
- viewer 禁止写接口
- 提现申请人与审核人分离
- 提现申请预占可用余额
- 幂等键防止重复提现 / 重复结算
- 收益修正采用追加流水，不覆盖原流水
- 结算仅纳入已支付且未结算的提现
- 所有关键变更写入审计日志

本系统只登记外部支付凭证，不执行真实转账。
