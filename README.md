# OPS Admin — React + Go + SQLite

当前版本已经把管理端改造成 React 技术栈，Go 后端和 SQLite 数据库继续沿用，不需要安装 MySQL。

## 技术栈

- 前端源码：React 18 + TypeScript + Vite + Ant Design 5
- 后端：Go 1.23 标准库 HTTP API
- 数据库：SQLite
- Windows 发布：单个 `ops-admin.exe` + 静态前端资源

## Windows 直接运行

Windows 10/11 不需要安装 Python、Node.js、Go 或 MySQL：

1. 解压整个目录。
2. 双击 `start-windows.bat`。
3. 浏览器打开 `http://127.0.0.1:8000`。

演示账号：

- 管理员：`admin / DemoAdmin2026!`
- 运营：`operator / DemoOperator2026!`
- 财务：`finance / DemoFinance2026!`
- 只读：`viewer / DemoViewer2026!`

数据库默认保存在 `data/ops.db`。首次启动且数据库为空时会写入合成演示数据。

## React 前端开发

前端工程位于 `frontend/`：

```bash
cd frontend
npm install
npm run dev
```

Vite 开发服务器会把 `/api` 代理到 `http://127.0.0.1:8000`。先在项目根目录运行 Go 服务：

```bash
go run ./cmd/server --db data/ops.db --demo --addr 127.0.0.1:8000
```

生产构建：

```bash
cd frontend
npm run build
```

构建结果直接写入根目录 `web/`，由 Go 服务同源托管。真实 Vite 构建会使用 React 18 和 Ant Design 5；发布包同时保留已编译静态资源，因此普通 Windows 使用者无需安装 Node。

> 当前执行环境无法访问 npm registry，所以交付包中的可立即运行静态资源使用同一套 React/TypeScript 业务代码生成，并内置本地 React 运行时；`frontend/package.json`、Vite 配置以及 Ant Design 入口均已保留，联网开发环境执行 `npm install && npm run build` 即可生成标准 Vite/Ant Design 发布包。

## 已改造页面

- 登录 / 会话鉴权
- 运营 Dashboard
- 运营监控
- 员工、小组、实名人员、实名渠道
- 聊天账号
- 收益明细与收益报表
- 提现申请、财务审核、驳回、支付登记
- 实名人员对账、结算账单
- 应用配置、黑名单、运营渠道、平台
- 系统用户与操作日志

## 权限与账务规则

仍由 Go 后端强制执行：

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

## 目录

```text
cmd/server/               Go 入口
internal/app/             API、鉴权、业务逻辑
internal/sqlite3driver/   SQLite 驱动
backend/schema.sql        数据库结构
frontend/                 React + TypeScript + Vite + Ant Design 源码
web/                      已构建、可直接运行的前端静态资源
data/                     SQLite 数据目录
docs/                     验收记录
```
