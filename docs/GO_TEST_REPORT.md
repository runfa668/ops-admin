# Go 版验收记录

验收日期：2026-09-14

已完成：

- Linux Go 1.23 编译成功。
- Windows amd64 `ops-admin.exe` 交叉编译成功，PE32+ 可执行文件。
- 全新 SQLite 数据库初始化和演示数据写入成功。
- `/api/health` 返回正常。
- `admin / DemoAdmin2026!` 登录成功，会话 Cookie 和 CSRF token 正常。
- `/api/meta`、`/api/dashboard`、`/api/accounts`、`/api/withdrawals` 读取成功。
- 收益写入成功并生成流水。
- viewer 写入收益返回 HTTP 403。
- admin 创建提现后自审返回 HTTP 403。
- finance 对该提现审核成功，状态 `pending -> approved`，`reviewedBy` 为财务账号。

当前限制：

- 当前环境无法直接运行 Windows PE，因此 Windows 可执行文件完成了交叉编译和格式检查，但没有在 Windows 实机启动。
- 当前环境没有安装 agent-browser CLI，因此本轮没有重新做浏览器自动化截图；前端静态文件未改，API 已通过 HTTP 烟测。
- `/web/...` 旧接口兼容层尚未完整移植，当前管理前端使用的 `/api/...` 为本轮重点。
