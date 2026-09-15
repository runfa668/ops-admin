const React = window.React;
const ReactDOM = window.ReactDOM;
const Antd = window.antd || {};
const h = React.createElement;
const money = (v) => `¥ ${Number(v || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
const today = () => new Date().toISOString().slice(0, 10);
const daysAgo = (n) => new Date(Date.now() - n * 86400000).toISOString().slice(0, 10);
const reqKey = () => `${Date.now()}-${Math.random().toString(16).slice(2)}-${Math.random().toString(16).slice(2)}`;
const roleName = { admin: '管理员', operator: '运营', finance: '财务', viewer: '只读' };
const stateName = { pending: '待审核', approved: '待支付', paid: '已支付', rejected: '已驳回', draft: '待结账' };
const routeName = { dashboard: '运营总览', monitor: '运营监控', earnings: '收益明细', withdrawals: '提现管理', reconciliation: '实名人员对账', settlements: '结账记录', employees: '员工管理', teams: '小组管理', holders: '实名人员', 'holder-channels': '实名渠道', accounts: '聊天账号', configs: '应用配置', blacklist: '黑名单', channels: '运营渠道', apps: '平台管理', users: '系统用户', 'audit-logs': '操作日志', 'reports/employees': '员工收益', 'reports/teams': '小组收益', 'reports/hourly': '小时收益', 'reports/comparison': '收益对比', 'reports/bans': '封号记录' };
const menus = [
    { title: '总览', items: [['dashboard', '运营总览'], ['monitor', '运营监控']] },
    { title: '账务', items: [['earnings', '收益明细'], ['withdrawals', '提现管理'], ['reconciliation', '实名人员对账'], ['settlements', '结账记录']] },
    { title: '人员与账号', items: [['employees', '员工管理'], ['teams', '小组管理'], ['holders', '实名人员'], ['holder-channels', '实名渠道'], ['accounts', '聊天账号']] },
    { title: '报表', items: [['reports/employees', '员工收益'], ['reports/teams', '小组收益'], ['reports/hourly', '小时收益'], ['reports/comparison', '收益对比'], ['reports/bans', '封号记录']] },
    { title: '系统', items: [['configs', '应用配置'], ['blacklist', '黑名单'], ['channels', '运营渠道'], ['apps', '平台管理'], ['users', '系统用户'], ['audit-logs', '操作日志']] }
];
async function api(path, method = 'GET', body, csrf = '') {
    const headers = {};
    if (body !== undefined)
        headers['Content-Type'] = 'application/json';
    if (method !== 'GET' && csrf)
        headers['X-CSRF-Token'] = csrf;
    const r = await fetch(path, { method, headers, credentials: 'same-origin', body: body === undefined ? undefined : JSON.stringify(body) });
    const v = await r.json().catch(() => ({ code: r.status, message: '服务器响应异常' }));
    if (!r.ok || v.code !== 0) {
        const e = new Error(v.message || '请求失败');
        e.status = r.status;
        throw e;
    }
    return v.data;
}
function Button(p) { if (Antd.Button) {
    const C = Antd.Button;
    return React.createElement(C, { type: p.type, danger: p.danger, disabled: p.disabled, onClick: p.onClick, htmlType: p.htmlType }, p.children);
} return React.createElement("button", { className: `a-btn ${p.type === 'primary' ? 'primary' : ''} ${p.danger ? 'danger' : ''}`, disabled: p.disabled, onClick: p.onClick, type: p.htmlType || 'button' }, p.children); }
function Card(p) { if (Antd.Card) {
    const C = Antd.Card;
    return React.createElement(C, { className: p.className, title: p.title }, p.children);
} return React.createElement("section", { className: `a-card ${p.className || ''}` },
    p.title && React.createElement("div", { className: "a-card-title" }, p.title),
    p.children); }
function Tag(p) { if (Antd.Tag) {
    const C = Antd.Tag;
    const color = { green: 'success', blue: 'processing', red: 'error', amber: 'warning' }[p.color];
    return React.createElement(C, { color: color }, p.children);
} return React.createElement("span", { className: `a-tag ${p.color || ''}` }, p.children); }
function Spin() { if (Antd.Spin) {
    const C = Antd.Spin;
    return React.createElement("div", { className: "spin-wrap" },
        React.createElement(C, { tip: "\u6B63\u5728\u8BFB\u53D6\u6570\u636E..." }));
} return React.createElement("div", { className: "spin-wrap" },
    React.createElement("span", { className: "spin" }),
    "\u6B63\u5728\u8BFB\u53D6\u6570\u636E..."); }
function StatusTag({ value, account = false }) { if (account) {
    return React.createElement(Tag, { color: value === 0 ? 'green' : value === 1 ? 'red' : 'gray' }, value === 0 ? '正常' : value === 1 ? '已封禁' : '已注销');
} return React.createElement(Tag, { color: value === 0 ? 'green' : 'gray' }, value === 0 ? '正常' : '停用'); }
function StateTag({ value }) { return React.createElement(Tag, { color: value === 'paid' ? 'green' : value === 'approved' ? 'blue' : value === 'rejected' ? 'red' : 'amber' }, stateName[value] || value); }
const entityForms = {
    channels: [['name', '名称', 'text'], ['status', '状态', 'status']],
    apps: [['name', '名称', 'text'], ['packageName', '应用包名', 'text'], ['status', '状态', 'status']],
    employees: [['name', '姓名', 'text'], ['channelId', '运营渠道', 'channels'], ['teamId', '所属小组', 'teams'], ['status', '状态', 'status']],
    teams: [['name', '小组名称', 'text'], ['channelId', '运营渠道', 'channels'], ['leaderId', '负责人', 'employees'], ['status', '状态', 'status']],
    holders: [['name', '姓名', 'text'], ['bakName', '备注名', 'text'], ['channelId', '运营渠道', 'channels'], ['accountChannelId', '实名渠道', 'holder-channels'], ['percent', '分成比例(%)', 'number'], ['status', '状态', 'status']],
    'holder-channels': [['name', '名称', 'text'], ['channelId', '运营渠道', 'channels'], ['percent', '登记比例(%)', 'number'], ['accountValidDays', '账号有效天数', 'number'], ['status', '状态', 'status']],
    accounts: [['chatAccountName', '账号昵称', 'text'], ['chatAccountBakName', '账号备注名', 'text'], ['chatAccountUid', '平台 UID', 'text'], ['appId', '平台', 'apps'], ['channelId', '运营渠道', 'channels'], ['employeeId', '负责人', 'employees'], ['accountHolderId', '实名人员', 'holders'], ['openingBalance', '期初余额', 'number'], ['giftAccount', '账号类型', 'gift'], ['accountStatus', '状态', 'accountStatus']],
    blacklist: [['uid', '账号 UID', 'text'], ['appId', '平台', 'apps'], ['reason', '加入原因', 'textarea'], ['status', '状态', 'status']],
    configs: [['accountId', '账号', 'accounts'], ['friendsCircleEndDate', '朋友圈到期日', 'date'], ['closeFriendsEndDate', '亲密好友到期日', 'date'], ['note', '备注', 'textarea'], ['status', '状态', 'status']]
};
const cols = {
    channels: [['name', '名称'], ['status', '状态', 'status'], ['createDate', '创建时间']],
    apps: [['name', '平台'], ['packageName', '包名'], ['status', '状态', 'status']],
    employees: [['name', '员工'], ['channelName', '运营渠道'], ['teamName', '小组'], ['accountCount', '账号数'], ['status', '状态', 'status']],
    teams: [['name', '小组'], ['channelName', '运营渠道'], ['leaderName', '负责人'], ['employeeCount', '人数'], ['status', '状态', 'status']],
    holders: [['name', '实名人员'], ['bakName', '备注'], ['channelName', '运营渠道'], ['accountChannelName', '实名渠道'], ['percent', '分成比例', 'percent'], ['status', '状态', 'status']],
    'holder-channels': [['name', '渠道'], ['channelName', '运营渠道'], ['percent', '登记比例', 'percent'], ['accountValidDays', '有效天数'], ['status', '状态', 'status']],
    accounts: [['chatAccountName', '账号'], ['chatAccountUid', 'UID'], ['appName', '平台'], ['employeeName', '负责人'], ['holderName', '实名人员'], ['total', '总余额', 'money'], ['available', '可用余额', 'money'], ['accountStatus', '状态', 'accountStatus']],
    blacklist: [['chatAccountUid', 'UID'], ['appName', '平台'], ['reason', '原因'], ['status', '状态', 'status'], ['createDate', '创建时间']],
    configs: [['chatAccountName', '账号'], ['appName', '平台'], ['employeeName', '负责人'], ['friendsCircleEndDate', '朋友圈到期'], ['closeFriendsEndDate', '亲密好友到期'], ['status', '状态', 'status']],
    earnings: [['day', '日期'], ['hour', '小时'], ['chatAccountName', '账号'], ['appName', '平台'], ['employeeName', '员工'], ['teamName', '小组'], ['amount', '金额', 'money'], ['kind', '类型', 'earningKind'], ['sourceRef', '来源编号']],
    withdrawals: [['chatAccountName', '账号'], ['appName', '平台'], ['holderName', '实名人员'], ['amount', '提现金额', 'money'], ['holderShare', '实名分成', 'money'], ['state', '状态', 'state'], ['paymentRef', '支付凭证'], ['createDate', '申请时间']],
    settlements: [['holderName', '实名人员'], ['amount', '结算金额', 'money'], ['startDate', '开始日期'], ['endDate', '结束日期'], ['state', '状态', 'state'], ['paymentRef', '支付凭证'], ['createDate', '生成时间']],
    reconciliation: [['name', '实名人员'], ['channelName', '运营渠道'], ['percent', '分成比例', 'percent'], ['paidTotal', '已支付提现', 'money'], ['unbilledShare', '待生成账单', 'money'], ['waitingPayTotal', '待结账', 'money'], ['finishedTotal', '已结账', 'money']],
    users: [['username', '用户名'], ['displayName', '显示名称'], ['role', '角色', 'role'], ['active', '状态', 'active'], ['createDate', '创建时间']],
    'audit-logs': [['actor', '操作人'], ['action', '动作'], ['entity', '对象'], ['entityId', '对象ID'], ['detail', '详情'], ['createDate', '时间']],
    monitor: [['chatAccountName', '账号'], ['appName', '平台'], ['employeeName', '负责人'], ['available', '可用余额', 'money'], ['lastSeen', '最后上报'], ['alerts', '异常', 'alerts'], ['accountStatus', '状态', 'accountStatus']],
    'reports/employees': [['name', '员工'], ['amount', '收益', 'money'], ['accountCount', '账号数'], ['entryCount', '流水数']],
    'reports/teams': [['name', '小组'], ['amount', '收益', 'money'], ['accountCount', '账号数'], ['entryCount', '流水数']],
    'reports/hourly': [['name', '小时'], ['amount', '收益', 'money'], ['accountCount', '账号数'], ['entryCount', '流水数']],
    'reports/comparison': [['chatAccountName', '账号'], ['appName', '平台'], ['employeeName', '员工'], ['current', '本期', 'money'], ['previous', '上期', 'money'], ['difference', '变动', 'money'], ['changePercent', '变动率', 'change']],
    'reports/bans': [['chatAccountName', '账号'], ['appName', '平台'], ['employeeName', '员工'], ['balanceAtBan', '封禁时余额', 'money'], ['reason', '原因'], ['createDate', '时间']]
};
class App extends React.Component {
    constructor(p) {
        super(p);
        this.onHash = () => { const r = (location.hash.slice(1) || 'dashboard').split('?')[0]; this.setState({ route: routeName[r] ? r : 'dashboard', page: 1, keyword: '', status: '' }, () => this.load()); };
        this.bootstrap = async () => { try {
            const u = await api('/api/auth/me');
            this.setState({ user: u, route: (location.hash.slice(1) || 'dashboard').split('?')[0] || 'dashboard' }, async () => { await this.refreshMeta(); await this.load(); });
        }
        catch (_a) {
            this.setState({ loading: false, user: null });
        } };
        this.refreshMeta = async () => { const m = await api('/api/meta'); this.setState({ meta: m }); };
        this.load = async () => { if (!this.state.user)
            return; this.setState({ loading: true, error: '' }); try {
            const s = this.state;
            let q = new URLSearchParams({ page: String(s.page), pageSize: String(s.pageSize) });
            if (s.keyword)
                q.set('keyword', s.keyword);
            if (s.channelId)
                q.set('channelId', s.channelId);
            if (s.status)
                q.set(['withdrawals', 'settlements'].includes(s.route) ? 'state' : 'status', s.status);
            if (['dashboard', 'earnings', 'withdrawals'].includes(s.route) || s.route.startsWith('reports/')) {
                q.set('startDate', s.startDate);
                q.set('endDate', s.endDate);
            }
            const d = await api(`/api/${s.route}?${q}`);
            this.setState({ data: d, loading: false });
        }
        catch (e) {
            if (e.status === 401) {
                this.setState({ user: null, loading: false });
            }
            else
                this.setState({ error: e.message, loading: false });
        } };
        this.login = async (e) => { e.preventDefault(); const f = new FormData(e.currentTarget); try {
            const u = await api('/api/auth/login', 'POST', { username: f.get('username'), password: f.get('password') });
            this.setState({ user: u }, async () => { await this.refreshMeta(); location.hash = 'dashboard'; await this.load(); });
        }
        catch (err) {
            this.setState({ error: err.message });
        } };
        this.logout = async () => { var _a; try {
            await api('/api/auth/logout', 'POST', {}, ((_a = this.state.user) === null || _a === void 0 ? void 0 : _a.csrfToken) || '');
        }
        finally {
            this.setState({ user: null, data: null, meta: {} });
        } };
        this.go = (r) => { location.hash = r; };
        this.open = (kind, row) => this.setState({ modal: { kind, row: row || null, key: reqKey() } });
        this.close = () => this.setState({ modal: null });
        this.mutate = async (path, method, body) => { var _a; try {
            await api(path, method, body, ((_a = this.state.user) === null || _a === void 0 ? void 0 : _a.csrfToken) || '');
            this.close();
            await this.refreshMeta();
            await this.load();
        }
        catch (e) {
            alert(e.message);
        } };
        this.saveEntity = async (e) => { var _a, _b; e.preventDefault(); const m = this.state.modal; const f = new FormData(e.currentTarget); const body = {}; for (const [k, , type] of entityForms[this.state.route] || []) {
            let v = f.get(k);
            if (['number', 'status', 'gift', 'accountStatus'].includes(type) || !isNaN(Number(v)) && ['channelId', 'teamId', 'leaderId', 'appId', 'employeeId', 'accountHolderId', 'accountChannelId', 'accountId'].includes(k)) {
                v = v === '' ? null : Number(v);
            }
            body[k] = v;
        } if (((_a = m.row) === null || _a === void 0 ? void 0 : _a.version) != null)
            body.version = m.row.version; const id = (_b = m.row) === null || _b === void 0 ? void 0 : _b.id; await this.mutate(`/api/${this.state.route}${id ? '/' + id : ''}`, id ? 'PUT' : 'POST', body); };
        this.deleteEntity = async (row) => { const run = () => this.mutate(`/api/${this.state.route}/${row.id}?version=${row.version || 0}`, 'DELETE', undefined); if (Antd.Modal && Antd.Modal.confirm) { Antd.Modal.confirm({ title: '确定删除这条记录？', content: '删除后不可恢复。', okText: '删除', cancelText: '取消', okButtonProps: { danger: true }, onOk: run }); return; } if (confirm('确定删除这条记录？')) await run(); };
        this.createSpecial = () => { const r = this.state.route; if (entityForms[r])
            this.open('entity');
        else if (r === 'earnings')
            this.open('earning');
        else if (r === 'withdrawals')
            this.open('withdrawal');
        else if (r === 'settlements')
            this.open('settlement');
        else if (r === 'users')
            this.open('user'); };
        this.saveSpecial = async (e) => { e.preventDefault(); const f = new FormData(e.currentTarget), m = this.state.modal, kind = m.kind; let path = '', method = 'POST', body = {}; if (kind === 'earning') {
            path = '/api/earnings';
            body = { accountId: Number(f.get('accountId')), day: f.get('day'), hour: Number(f.get('hour')), amount: Number(f.get('amount')), sourceRef: f.get('sourceRef'), note: f.get('note') };
        } if (kind === 'adjust') {
            path = `/api/earnings/${m.row.id}/adjust`;
            body = { amount: Number(f.get('amount')), sourceRef: f.get('sourceRef'), reason: f.get('reason') };
        } if (kind === 'withdrawal') {
            path = '/api/withdrawals';
            body = { accountId: Number(f.get('accountId')), amount: Number(f.get('amount')), fee: Number(f.get('fee') || 0), idempotencyKey: m.key, note: f.get('note') };
        } if (kind === 'withdrawTransition') {
            path = `/api/withdrawals/${m.row.id}/transition`;
            body = { action: m.action, version: m.row.version, reason: f.get('reason') || '', paymentRef: f.get('paymentRef') || '' };
        } if (kind === 'settlement') {
            path = '/api/settlements';
            body = { holderId: Number(f.get('holderId')), startDate: f.get('startDate'), endDate: f.get('endDate'), idempotencyKey: m.key };
        } if (kind === 'settlePay') {
            path = `/api/settlements/${m.row.id}/pay`;
            body = { version: m.row.version, paymentRef: f.get('paymentRef') };
        } if (kind === 'user') {
            path = '/api/users';
            body = { username: f.get('username'), displayName: f.get('displayName'), password: f.get('password'), role: f.get('role') };
        } if (kind === 'heartbeat') {
            path = '/api/ingest/heartbeat';
            body = { accountId: m.row.id };
        } await this.mutate(path, method, body); };
        this.action = (act, row) => { if (act === 'edit')
            this.open('entity', row);
        else if (act === 'delete')
            this.deleteEntity(row);
        else if (act === 'adjust')
            this.setState({ modal: { kind: 'adjust', row, key: reqKey() } });
        else if (['approve', 'reject', 'pay'].includes(act))
            this.setState({ modal: { kind: 'withdrawTransition', row, action: act, key: reqKey() } });
        else if (act === 'settlePay')
            this.setState({ modal: { kind: 'settlePay', row, key: reqKey() } });
        else if (act === 'heartbeat')
            this.setState({ modal: { kind: 'heartbeat', row, key: reqKey() } });
        else if (act === 'ban')
            this.mutate(`/api/accounts/${row.id}/ban`, 'POST', { reason: '后台手动封禁', version: row.version });
        else if (act === 'unban')
            this.mutate(`/api/accounts/${row.id}/unban`, 'POST', { reason: '后台手动解封', version: row.version });
        else if (act === 'disableUser' && confirm('确定停用此用户？'))
            this.mutate(`/api/users/${row.id}/disable`, 'POST', {}); };
        this.canWrite = () => { var _a; return ['admin', 'operator'].includes(((_a = this.state.user) === null || _a === void 0 ? void 0 : _a.role) || ''); };
        this.canFinance = () => { var _a; return ['admin', 'finance'].includes(((_a = this.state.user) === null || _a === void 0 ? void 0 : _a.role) || ''); };
        this.state = { user: null, meta: {}, route: 'dashboard', loading: true, data: null, page: 1, pageSize: 10, keyword: '', status: '', channelId: '', startDate: daysAgo(6), endDate: today(), modal: null, error: '' };
    }
    componentDidMount() { window.addEventListener('hashchange', this.onHash); this.bootstrap(); }
    componentWillUnmount() { window.removeEventListener('hashchange', this.onHash); }
    renderLogin() { return React.createElement("div", { className: "login" },
        React.createElement("div", { className: "login-brand" },
            React.createElement("div", { className: "logo" },
                "OPS ",
                React.createElement("span", null, "WORKSPACE")),
            React.createElement("div", null,
                React.createElement("p", { className: "eyebrow" }, "OPERATIONS, IN FOCUS."),
                React.createElement("h1", null,
                    "\u8FD0\u8425\u6709\u5E8F\u3002",
                    React.createElement("br", null),
                    "\u6536\u76CA\u6709\u6570\u3002"),
                React.createElement("p", { className: "lead" }, "\u4EBA\u5458\u3001\u8D26\u53F7\u3001\u6536\u76CA\u3001\u63D0\u73B0\u548C\u5BA1\u8BA1\uFF0C\u4E00\u5957\u5DE5\u4F5C\u53F0\u96C6\u4E2D\u7BA1\u7406\u3002")),
            React.createElement("small", null, "REACT ADMIN \u00B7 GO API \u00B7 SQLITE")),
        React.createElement("div", { className: "login-panel" },
            React.createElement("form", { className: "login-box", onSubmit: this.login },
                React.createElement("p", { className: "eyebrow" }, "WELCOME BACK"),
                React.createElement("h2", null, "\u767B\u5F55\u8FD0\u8425\u5DE5\u4F5C\u53F0"),
                React.createElement("label", null,
                    "\u7528\u6237\u540D",
                    React.createElement("input", { name: "username", defaultValue: "admin", autoComplete: "username", required: true })),
                React.createElement("label", null,
                    "\u5BC6\u7801",
                    React.createElement("input", { name: "password", type: "password", defaultValue: "DemoAdmin2026!", autoComplete: "current-password", required: true })),
                this.state.error && React.createElement("div", { className: "form-error" }, this.state.error),
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u767B\u5F55"),
                React.createElement("p", { className: "hint" }, "\u6F14\u793A\uFF1Aadmin / DemoAdmin2026!")))); }
    renderSidebar() { var _a; const role = (_a = this.state.user) === null || _a === void 0 ? void 0 : _a.role; return React.createElement("aside", { className: "sidebar" },
        React.createElement("div", { className: "brand" },
            "OPS ",
            React.createElement("span", null, "WORKSPACE")),
        React.createElement("nav", null, menus.map((g) => React.createElement("div", { className: "menu-group", key: g.title },
            React.createElement("div", { className: "menu-title" }, g.title),
            g.items.filter((x) => x[0] !== 'users' || role === 'admin').map((x) => React.createElement("button", { className: `menu-item ${this.state.route === x[0] ? 'active' : ''}`, onClick: () => this.go(x[0]), key: x[0] }, x[1])))))); }
    renderHeader() { const u = this.state.user; return React.createElement("header", { className: "topbar" },
        React.createElement("div", null,
            React.createElement("span", { className: "crumb" }, "\u8FD0\u8425\u5DE5\u4F5C\u53F0 / "),
            React.createElement("strong", null, routeName[this.state.route])),
        React.createElement("div", { className: "userbox" },
            React.createElement("span", null, u.companyName),
            React.createElement(Tag, { color: "blue" }, roleName[u.role]),
            React.createElement("strong", null, u.name),
            React.createElement(Button, { onClick: this.logout }, "\u9000\u51FA"))); }
    renderDashboard() { const d = this.state.data || {}; const k = [['账号总余额', money(d.balance)], ['可用余额', money(d.available)], ['区间收益', money(d.income)], ['已支付提现', money(d.withdrawn)], ['运营账号', `${d.activeAccounts || 0} / ${d.accountCount || 0}`], ['待处理提现', String(d.pendingCount || 0)]]; return React.createElement("div", null,
        React.createElement("div", { className: "kpis" }, k.map(x => React.createElement(Card, { key: x[0] },
            React.createElement("div", { className: "kpi-label" }, x[0]),
            React.createElement("div", { className: "kpi-value" }, x[1])))),
        React.createElement("div", { className: "grid2" },
            React.createElement(Card, { title: "\u6536\u76CA / \u63D0\u73B0\u8D8B\u52BF" },
                React.createElement("div", { className: "trend" }, (d.series || []).map((x) => React.createElement("div", { className: "trend-row", key: x.day },
                    React.createElement("span", null, x.day.slice(5)),
                    React.createElement("div", { className: "bar income", style: { width: `${Math.min(100, Number(x.income || 0) / 10)}%` } }),
                    React.createElement("b", null, money(x.income)),
                    React.createElement("small", null,
                        "\u63D0\u73B0 ",
                        money(x.withdraw)))))),
            React.createElement(Card, { title: "\u5458\u5DE5\u6536\u76CA\u6392\u884C" }, (d.ranking || []).map((x, i) => React.createElement("div", { className: "rank", key: x.employeeId },
                React.createElement("span", null, i + 1),
                React.createElement("strong", null, x.employeeName),
                React.createElement("b", null, money(x.amount)))))),
        React.createElement(Card, { title: "\u5F02\u5E38\u5F85\u529E" },
            React.createElement("div", { className: "alerts" }, Object.entries(d.alerts || {}).map(([k, v]) => React.createElement("div", { key: k },
                React.createElement("b", null, v),
                React.createElement("span", null, { banned: '封禁账号', missing_income: '今日漏统', no_employee: '未分配员工', no_holder: '未绑定实名', stale_heartbeat: '心跳超时' }[k] || k)))))); }
    valueCell(row, c) { const [k, , t] = c, v = row[k]; if (t === 'money')
        return money(v); if (t === 'percent')
        return `${Number(v || 0).toFixed(2)}%`; if (t === 'status')
        return React.createElement(StatusTag, { value: v }); if (t === 'accountStatus')
        return React.createElement(StatusTag, { value: v, account: true }); if (t === 'state')
        return React.createElement(StateTag, { value: v }); if (t === 'role')
        return roleName[v] || v; if (t === 'active')
        return React.createElement(Tag, { color: v ? 'green' : 'gray' }, v ? '启用' : '停用'); if (t === 'earningKind')
        return v === 'adjustment' ? React.createElement(Tag, { color: "amber" }, "\u4FEE\u6B63") : React.createElement(Tag, { color: "green" }, "\u6536\u5165"); if (t === 'alerts')
        return (v || []).length ? (v || []).map((a) => React.createElement(Tag, { color: "red", key: a }, a)) : React.createElement(Tag, { color: "green" }, "\u6B63\u5E38"); if (t === 'change')
        return v == null ? '--' : `${v > 0 ? '+' : ''}${v}%`; return v === null || v === undefined || v === '' ? '--' : String(v); }
    actions(row) { const r = this.state.route, u = this.state.user; let a = []; if (entityForms[r] && this.canWrite()) {
        a.push(['edit', '编辑']);
        if (r === 'accounts') {
            if (row.accountStatus === 0)
                a.push(['ban', '封禁']);
            if (row.accountStatus === 1)
                a.push(['unban', '解封']);
        }
        else if (u.role === 'admin')
            a.push(['delete', '删除']);
    } if (r === 'earnings' && this.canWrite() && row.kind === 'income')
        a.push(['adjust', '修正']); if (r === 'withdrawals' && this.canFinance()) {
        if (row.state === 'pending' && row.createdBy !== u.id)
            a.push(['approve', '审核']);
        if (['pending', 'approved'].includes(row.state))
            a.push(['reject', '驳回']);
        if (row.state === 'approved')
            a.push(['pay', '登记支付']);
    } if (r === 'settlements' && this.canFinance() && row.state === 'draft')
        a.push(['settlePay', '确认结账']); if (r === 'monitor' && this.canWrite())
        a.push(['heartbeat', '上报心跳']); if (r === 'users' && u.role === 'admin' && row.id !== u.id && row.active)
        a.push(['disableUser', '停用']); return a; }
    renderTable() { var _a; const d = this.state.data || { list: [] }, list = d.list || [], cs = cols[this.state.route] || []; return React.createElement(Card, null,
        React.createElement("div", { className: "table-head" },
            React.createElement("strong", null, routeName[this.state.route]),
            React.createElement("span", null, (_a = d.total) !== null && _a !== void 0 ? _a : list.length,
                " \u6761\u8BB0\u5F55")),
        React.createElement("div", { className: "table-scroll" },
            React.createElement("table", null,
                React.createElement("thead", null,
                    React.createElement("tr", null,
                        cs.map((c) => React.createElement("th", { key: c[0] }, c[1])),
                        React.createElement("th", null, "\u64CD\u4F5C"))),
                React.createElement("tbody", null, list.length ? list.map((row) => React.createElement("tr", { key: row.id },
                    cs.map((c) => React.createElement("td", { key: c[0] }, this.valueCell(row, c))),
                    React.createElement("td", null,
                        React.createElement("div", { className: "row-actions" }, this.actions(row).map((a) => React.createElement("button", { key: a[0], className: ['delete', 'ban', 'reject', 'disableUser'].includes(a[0]) ? 'danger-link' : '', onClick: () => this.action(a[0], row) }, a[1])))))) : React.createElement("tr", null,
                    React.createElement("td", { colSpan: cs.length + 1 },
                        React.createElement("div", { className: "empty" }, "\u6682\u65E0\u5339\u914D\u6570\u636E")))))),
        d.total != null && React.createElement("div", { className: "pager" },
            React.createElement("span", null,
                "\u5171 ",
                d.total,
                " \u6761"),
            React.createElement("select", { value: this.state.pageSize, onChange: (e) => this.setState({ pageSize: Number(e.target.value), page: 1 }, () => this.load()) },
                React.createElement("option", null, "10"),
                React.createElement("option", null, "20"),
                React.createElement("option", null, "50")),
            React.createElement(Button, { disabled: this.state.page <= 1, onClick: () => this.setState({ page: this.state.page - 1 }, () => this.load()) }, "\u4E0A\u4E00\u9875"),
            React.createElement("b", null, this.state.page),
            React.createElement(Button, { disabled: this.state.page * this.state.pageSize >= d.total, onClick: () => this.setState({ page: this.state.page + 1 }, () => this.load()) }, "\u4E0B\u4E00\u9875"))); }
    createAllowed() { const r = this.state.route, u = this.state.user; return !!(entityForms[r] && this.canWrite() || r === 'earnings' && this.canWrite() || r === 'withdrawals' && ['admin', 'operator', 'finance'].includes(u.role) || r === 'settlements' && this.canFinance() || r === 'users' && u.role === 'admin'); }
    renderFilters() { var _a; const s = this.state; const date = ['dashboard', 'earnings', 'withdrawals'].includes(s.route) || s.route.startsWith('reports/'); const state = s.route === 'withdrawals' ? ['', 'pending', 'approved', 'paid', 'rejected'] : s.route === 'settlements' ? ['', 'draft', 'paid'] : null; return React.createElement("div", { className: "filters" },
        React.createElement("input", { placeholder: "\u641C\u7D22\u540D\u79F0\u6216\u8D26\u53F7", value: s.keyword, onChange: (e) => this.setState({ keyword: e.target.value }) }),
        !['apps', 'channels', 'users', 'audit-logs', 'blacklist', 'settlements'].includes(s.route) && React.createElement("select", { value: s.channelId, onChange: (e) => this.setState({ channelId: e.target.value, page: 1 }, () => this.load()) },
            React.createElement("option", { value: "" }, "\u5168\u90E8\u6E20\u9053"),
            (((_a = s.meta.channels) === null || _a === void 0 ? void 0 : _a.list) || []).map((x) => React.createElement("option", { key: x.id, value: x.id }, x.name))),
        state && React.createElement("select", { value: s.status, onChange: (e) => this.setState({ status: e.target.value, page: 1 }, () => this.load()) }, state.map((x) => React.createElement("option", { value: x, key: x }, x ? stateName[x] : '全部状态'))),
        date && React.createElement("span", { className: "date-inline" },
            React.createElement("input", { type: "date", value: s.startDate, onChange: (e) => this.setState({ startDate: e.target.value }) }),
            React.createElement("span", null, "\u81F3"),
            React.createElement("input", { type: "date", value: s.endDate, onChange: (e) => this.setState({ endDate: e.target.value }) })),
        React.createElement(Button, { onClick: () => this.setState({ page: 1 }, () => this.load()) }, "\u67E5\u8BE2"),
        React.createElement(Button, { onClick: () => this.setState({ keyword: '', status: '', channelId: '', page: 1, startDate: daysAgo(6), endDate: today() }, () => this.load()) }, "\u91CD\u7F6E")); }
    renderMain() { return React.createElement("main", { className: "main" },
        React.createElement("div", { className: "page-title" },
            React.createElement("div", null,
                React.createElement("h1", null, routeName[this.state.route]),
                React.createElement("p", null, "\u6570\u636E\u6309\u516C\u53F8\u9694\u79BB\uFF0C\u5173\u952E\u64CD\u4F5C\u4FDD\u7559\u5BA1\u8BA1\u8BB0\u5F55\u3002")),
            React.createElement("div", null,
                React.createElement(Button, { onClick: () => this.load() }, "\u5237\u65B0"),
                this.createAllowed() && React.createElement(Button, { type: "primary", onClick: this.createSpecial }, this.state.route === 'earnings' ? '收益入账' : this.state.route === 'withdrawals' ? '申请提现' : this.state.route === 'settlements' ? '生成账单' : this.state.route === 'users' ? '新增用户' : '新增'))),
        this.state.meta.demo && React.createElement("div", { className: "demo" }, "\u672C\u5730\u6F14\u793A\u73AF\u5883\uFF1A\u9884\u7F6E\u6570\u636E\u5747\u4E3A\u5408\u6210\u6570\u636E\uFF0C\u4E0D\u6267\u884C\u771F\u5B9E\u6253\u6B3E\u3002"),
        this.renderFilters(),
        this.state.loading ? React.createElement(Spin, null) : this.state.error ? React.createElement(Card, null,
            React.createElement("div", { className: "form-error" }, this.state.error),
            React.createElement(Button, { onClick: () => this.load() }, "\u91CD\u65B0\u52A0\u8F7D")) : this.state.route === 'dashboard' ? this.renderDashboard() : this.renderTable()); }
    options(source) { var _a; return ((_a = this.state.meta[source]) === null || _a === void 0 ? void 0 : _a.list) || []; }
    fieldInput(f, row) { var _a, _b; const [key, label, type] = f; let v = (_b = (_a = row === null || row === void 0 ? void 0 : row[key]) !== null && _a !== void 0 ? _a : (key === 'uid' ? row === null || row === void 0 ? void 0 : row.chatAccountUid : undefined)) !== null && _b !== void 0 ? _b : ''; if (type === 'status')
        return React.createElement("label", { key: key },
            label,
            React.createElement("select", { name: key, defaultValue: String(v || 0) },
                React.createElement("option", { value: "0" }, "\u6B63\u5E38"),
                React.createElement("option", { value: "1" }, "\u505C\u7528"))); if (type === 'gift')
        return React.createElement("label", { key: key },
            label,
            React.createElement("select", { name: key, defaultValue: String(v || 0) },
                React.createElement("option", { value: "0" }, "\u666E\u901A\u8D26\u53F7"),
                React.createElement("option", { value: "1" }, "\u793C\u7269\u8D26\u53F7"))); if (type === 'accountStatus')
        return React.createElement("label", { key: key },
            label,
            React.createElement("select", { name: key, defaultValue: String(v || 0) },
                React.createElement("option", { value: "0" }, "\u6B63\u5E38"),
                React.createElement("option", { value: "2" }, "\u6CE8\u9500"))); if (['channels', 'teams', 'employees', 'holders', 'holder-channels', 'apps', 'accounts'].includes(type))
        return React.createElement("label", { key: key },
            label,
            React.createElement("select", { name: key, defaultValue: v == null ? '' : String(v) },
                React.createElement("option", { value: "" }, "\u8BF7\u9009\u62E9"),
                this.options(type).map((x) => React.createElement("option", { value: x.id, key: x.id }, type === 'accounts' ? `${x.chatAccountName} / 可用 ${money(x.available)}` : x.name)))); if (type === 'textarea')
        return React.createElement("label", { key: key, className: "full" },
            label,
            React.createElement("textarea", { name: key, defaultValue: v })); return React.createElement("label", { key: key },
        label,
        React.createElement("input", { name: key, type: type, step: type === 'number' ? '0.01' : undefined, defaultValue: v })); }
    renderModal() { const m = this.state.modal; if (!m)
        return null; let title = '操作', body = null; const close = React.createElement(Button, { onClick: this.close }, "\u53D6\u6D88"); if (m.kind === 'entity') {
        title = m.row ? '编辑记录' : '新增记录';
        body = React.createElement("form", { onSubmit: this.saveEntity },
            React.createElement("div", { className: "form-grid" }, (entityForms[this.state.route] || []).map((f) => this.fieldInput(f, m.row))),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u4FDD\u5B58")));
    }
    else if (m.kind === 'earning') {
        title = '收益入账';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "form-grid" },
                React.createElement("label", null,
                    "\u8D26\u53F7",
                    React.createElement("select", { name: "accountId", required: true }, this.options('accounts').map((x) => React.createElement("option", { value: x.id, key: x.id },
                        x.chatAccountName,
                        " / ",
                        money(x.available))))),
                React.createElement("label", null,
                    "\u65E5\u671F",
                    React.createElement("input", { name: "day", type: "date", defaultValue: today(), required: true })),
                React.createElement("label", null,
                    "\u5C0F\u65F6",
                    React.createElement("input", { name: "hour", type: "number", min: "0", max: "23", defaultValue: new Date().getHours(), required: true })),
                React.createElement("label", null,
                    "\u91D1\u989D",
                    React.createElement("input", { name: "amount", type: "number", step: "0.01", min: "0.01", required: true })),
                React.createElement("label", { className: "full" },
                    "\u6765\u6E90\u7F16\u53F7",
                    React.createElement("input", { name: "sourceRef", defaultValue: `MANUAL-${reqKey()}`, minLength: 8, required: true })),
                React.createElement("label", { className: "full" },
                    "\u5907\u6CE8",
                    React.createElement("textarea", { name: "note" }))),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u786E\u8BA4\u5165\u8D26")));
    }
    else if (m.kind === 'adjust') {
        title = '修正收益';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "form-grid" },
                React.createElement("label", null,
                    "\u4FEE\u6B63\u91D1\u989D",
                    React.createElement("input", { name: "amount", type: "number", step: "0.01", required: true })),
                React.createElement("label", null,
                    "\u6765\u6E90\u7F16\u53F7",
                    React.createElement("input", { name: "sourceRef", defaultValue: `ADJUST-${reqKey()}`, minLength: 8, required: true })),
                React.createElement("label", { className: "full" },
                    "\u4FEE\u6B63\u539F\u56E0",
                    React.createElement("textarea", { name: "reason", minLength: 3, required: true }))),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u63D0\u4EA4\u4FEE\u6B63")));
    }
    else if (m.kind === 'withdrawal') {
        title = '申请提现';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "form-grid" },
                React.createElement("label", null,
                    "\u8D26\u53F7",
                    React.createElement("select", { name: "accountId", required: true }, this.options('accounts').filter((x) => x.accountStatus === 0).map((x) => React.createElement("option", { value: x.id, key: x.id },
                        x.chatAccountName,
                        " / \u53EF\u7528 ",
                        money(x.available))))),
                React.createElement("label", null,
                    "\u91D1\u989D",
                    React.createElement("input", { name: "amount", type: "number", step: "0.01", min: "0.01", required: true })),
                React.createElement("label", null,
                    "\u624B\u7EED\u8D39",
                    React.createElement("input", { name: "fee", type: "number", step: "0.01", min: "0", defaultValue: "0" })),
                React.createElement("label", { className: "full" },
                    "\u5907\u6CE8",
                    React.createElement("textarea", { name: "note" }))),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u63D0\u4EA4\u7533\u8BF7")));
    }
    else if (m.kind === 'withdrawTransition') {
        title = { approve: '审核提现', reject: '驳回提现', pay: '登记支付' }[m.action] || '提现操作';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "form-grid" },
                m.action === 'reject' && React.createElement("label", { className: "full" },
                    "\u9A73\u56DE\u539F\u56E0",
                    React.createElement("textarea", { name: "reason", minLength: 3, required: true })),
                m.action === 'pay' && React.createElement("label", { className: "full" },
                    "\u5916\u90E8\u652F\u4ED8\u51ED\u8BC1",
                    React.createElement("input", { name: "paymentRef", minLength: 6, required: true })),
                m.action === 'approve' && React.createElement("div", { className: "full info" }, "\u786E\u8BA4\u901A\u8FC7\u8BE5\u63D0\u73B0\u7533\u8BF7\uFF1F")),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u786E\u8BA4")));
    }
    else if (m.kind === 'settlement') {
        title = '生成结算账单';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "form-grid" },
                React.createElement("label", null,
                    "\u5B9E\u540D\u4EBA\u5458",
                    React.createElement("select", { name: "holderId", required: true }, this.options('holders').map((x) => React.createElement("option", { value: x.id, key: x.id }, x.name)))),
                React.createElement("label", null,
                    "\u5F00\u59CB\u65E5\u671F",
                    React.createElement("input", { name: "startDate", type: "date", defaultValue: daysAgo(30), required: true })),
                React.createElement("label", null,
                    "\u7ED3\u675F\u65E5\u671F",
                    React.createElement("input", { name: "endDate", type: "date", defaultValue: today(), required: true }))),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u751F\u6210\u8D26\u5355")));
    }
    else if (m.kind === 'settlePay') {
        title = '确认结账';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("label", null,
                "\u652F\u4ED8\u51ED\u8BC1",
                React.createElement("input", { name: "paymentRef", minLength: 6, required: true })),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u786E\u8BA4\u7ED3\u8D26")));
    }
    else if (m.kind === 'user') {
        title = '新增系统用户';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "form-grid" },
                React.createElement("label", null,
                    "\u7528\u6237\u540D",
                    React.createElement("input", { name: "username", minLength: 3, required: true })),
                React.createElement("label", null,
                    "\u663E\u793A\u540D\u79F0",
                    React.createElement("input", { name: "displayName", required: true })),
                React.createElement("label", null,
                    "\u89D2\u8272",
                    React.createElement("select", { name: "role" },
                        React.createElement("option", { value: "operator" }, "\u8FD0\u8425"),
                        React.createElement("option", { value: "finance" }, "\u8D22\u52A1"),
                        React.createElement("option", { value: "viewer" }, "\u53EA\u8BFB"),
                        React.createElement("option", { value: "admin" }, "\u7BA1\u7406\u5458"))),
                React.createElement("label", null,
                    "\u521D\u59CB\u5BC6\u7801",
                    React.createElement("input", { name: "password", type: "password", minLength: 12, required: true }))),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u521B\u5EFA")));
    }
    else if (m.kind === 'heartbeat') {
        title = '上报心跳';
        body = React.createElement("form", { onSubmit: this.saveSpecial },
            React.createElement("div", { className: "info" },
                "\u4E3A\u8D26\u53F7 ",
                m.row.chatAccountName,
                " \u5199\u5165\u4E00\u6B21\u6D4B\u8BD5\u5FC3\u8DF3\uFF0C\u4EC5\u7528\u4E8E\u9A8C\u8BC1\u76D1\u63A7\u63A5\u53E3\u3002"),
            React.createElement("div", { className: "modal-foot" },
                close,
                React.createElement(Button, { type: "primary", htmlType: "submit" }, "\u786E\u8BA4\u4E0A\u62A5")));
    } ; return React.createElement("div", { className: "modal-mask", onMouseDown: (e) => { if (e.target === e.currentTarget)
            this.close(); } },
        React.createElement("div", { className: "modal" },
            React.createElement("div", { className: "modal-head" },
                React.createElement("h2", null, title),
                React.createElement("button", { onClick: this.close }, "\u00D7")),
            React.createElement("div", { className: "modal-body" }, body))); }
    render() { if (!this.state.user)
        return this.renderLogin(); return React.createElement("div", { className: "shell" },
        this.renderSidebar(),
        React.createElement("div", { className: "workspace" },
            this.renderHeader(),
            this.renderMain()),
        this.renderModal()); }
}
ReactDOM.render(React.createElement(App, null), document.getElementById('root'));
