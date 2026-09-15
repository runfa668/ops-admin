# React 改造验收记录

日期：2026-09-14

## 本轮范围

- 管理后台从原生 JavaScript 页面改造为 React/TypeScript 结构。
- 增加 Vite 工程与 Ant Design 5 开发入口。
- Go API、Cookie/CSRF 权限体系以及 SQLite 数据层保持不变。
- Go 静态资源路由调整为标准 `/assets/* -> web/assets/*`，兼容 Vite 输出目录。
- Windows `ops-admin.exe` 已重新交叉编译。

## 验证结果

### Go / 静态资源

- Linux Go 编译：通过。
- Windows amd64 交叉编译：通过。
- `/` 首页：200。
- `/assets/app.js`：200。
- `/assets/style.css`：200。
- `/assets/vendor/react.min.js`：200。
- JavaScript `node --check`：通过。
- React TypeScript 运行时 `tsc` 编译：通过。

### React 页面

通过 Playwright 在隔离页面中加载本地 React/ReactDOM 与编译后的 TSX：

- 登录页正常渲染，无 page error。
- Dashboard 正常渲染。
- 左侧导航正常渲染。
- 员工管理菜单切换与表格渲染正常。
- 页面截图人工检查通过。

当前容器 Chromium 对本机 HTTP 地址有 `ERR_BLOCKED_BY_ADMINISTRATOR` 策略，因此无法从 Chromium 直接访问 `127.0.0.1` 完成原生网络 E2E；浏览器渲染验证和真实 Go API 验证分别执行。

### 真实 Go API 冒烟测试

使用临时 SQLite 数据库启动本轮 Go 服务并实际执行：

- operator 登录：通过。
- Dashboard / 员工列表读取：通过。
- 手工收益入账：通过。
- operator 提交提现：通过，状态 `pending`。
- finance 审核提现：通过，状态 `approved`。
- finance 登记外部支付：通过，状态 `paid`。
- viewer 尝试写入渠道：HTTP 403，通过。
- 新静态资源路径读取：通过。

## 已知说明

构建环境无法解析 npm registry，因此没有在本容器重新下载 Ant Design npm 包。`frontend/` 已配置 React 18、TypeScript、Vite 和 Ant Design 5；可直接运行的 `web/` 静态包使用相同 TSX 业务代码和本地 React 运行时生成。联网开发机运行 `npm install && npm run build` 后，会生成标准 Vite + Ant Design 构建结果。
