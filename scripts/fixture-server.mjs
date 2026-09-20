// Loopback-only stock-contract fixture. No tasks or external requests are executed.
import { createServer } from 'node:http';
import { randomUUID } from 'node:crypto';

const template = { jobGroup: 1, scheduleType: 'CRON', scheduleConf: '0 0/5 * * * ?', misfireStrategy: 'DO_NOTHING', author: 'demo', alarmEmail: '', executorRouteStrategy: 'FIRST', executorParam: '{"demo":true}', executorBlockStrategy: 'SERIAL_EXECUTION', executorTimeout: 30, executorFailRetryCount: 0, glueType: 'BEAN', childJobId: '', triggerStatus: 0 };
let jobs = [
  { ...template, id: 101, jobDesc: '示例账单同步', executorHandler: 'demoSyncHandler', triggerStatus: 1 },
  { ...template, id: 102, jobDesc: '示例状态巡检', executorHandler: 'demoCheckHandler', scheduleType: 'FIX_RATE', scheduleConf: '60' },
  { ...template, id: 201, jobGroup: 2, jobDesc: '示例报表生成', executorHandler: 'demoReportHandler' },
];
let groups = [
  { id: 1, appname: 'demo-executor', title: '示例执行器', addressType: 1, addressList: 'http://demo-node.invalid:9999/' },
  { id: 2, appname: 'report-executor', title: '报表执行器', addressType: 0, addressList: '' },
];
let users = [{ id: 1, username: 'admin', role: 1, permission: '', password: null }, { id: 2, username: 'operator', role: 0, permission: '1', password: null }];
const sessions = new Map(), requests = [];
const escape = value => String(value).replace(/[&"<>]/g, c => ({ '&': '&amp;', '"': '&quot;', '<': '&lt;', '>': '&gt;' }[c]));
const numeric = ['id', 'jobGroup', 'executorTimeout', 'executorFailRetryCount', 'addressType'];
const prefix = '/xxl-job-admin';
const server = createServer(async (req, res) => {
  const path = new URL(req.url, 'http://127.0.0.1').pathname;
  const json = value => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(value)); };
  if (path === '/__requests') return json(requests);
  if (path === '/__expire' && req.method === 'POST') { sessions.clear(); return json({ success: true }); }
  if (!path.startsWith(prefix)) { res.statusCode = 404; return res.end(); }
  let body = '';
  for await (const chunk of req) body += chunk;
  const form = Object.fromEntries(new URLSearchParams(body)), route = path.slice(prefix.length) || '/';
  const token = (req.headers.cookie || '').split(';').map(v => v.trim()).find(v => v.startsWith('XXL_JOB_LOGIN_IDENTITY='))?.split('=')[1];
  const username = sessions.get(token);
  requests.push({ method: req.method, path, form: 'password' in form ? { ...form, password: '[REDACTED]' } : form, username: username || null });
  if (requests.length > 1000) requests.shift();
  if (route === '/login') {
    if (req.method !== 'POST' || Object.keys(form).sort().join(',') !== 'password,userName' || !['admin', 'operator'].includes(form.userName) || form.password !== 'fixture-only-password') return json({ code: 500, msg: 'Invalid demo credentials' });
    const id = randomUUID(); sessions.set(id, form.userName);
    res.setHeader('Set-Cookie', `XXL_JOB_LOGIN_IDENTITY=${id}; Path=${prefix}; HttpOnly; SameSite=Lax`);
    return json({ code: 200, msg: null });
  }
  if (!username) { res.statusCode = 302; res.setHeader('Location', `${prefix}/toLogin`); return res.end(); }
  const authorized = g => username === 'admin' || g === 1;
  if (route === '/') {
    res.setHeader('Content-Type', 'text/html');
    return res.end(`<a href="${prefix}/jobinfo">Tasks</a>${username === 'admin' ? `<a href="${prefix}/jobgroup">Groups</a>` : ''}<span class="info-box-number">${jobs.length}</span><span class="info-box-number">42</span><span class="info-box-number">1</span>`);
  }
  if (route === '/jobinfo') {
    res.setHeader('Content-Type', 'text/html');
    return res.end(`<select id="jobGroup">${groups.filter(g => authorized(g.id)).map(g => `<option value="${g.id}">${escape(g.title)}</option>`).join('')}</select>`);
  }
  const page = rows => json({ recordsTotal: rows.length, recordsFiltered: rows.length, data: rows.slice(Number(form.start || 0), Number(form.start || 0) + Number(form.length || 25)) });
  if (Object.keys(form).some(k => ['jobCron', 'jobStatus', 'order', 'taskLimit', 'appName'].includes(k))) return json({ code: 500, msg: 'Custom fields are not stock 2.3.0' });
  if (route === '/jobgroup/pageList') return username === 'admin' ? page(groups) : json({ code: 500, msg: 'No permission' });
  if (route === '/chartInfo') {
    if (!/^\d{4}-\d{2}-\d{2} 00:00:00$/.test(form.startDate || '') || !/^\d{4}-\d{2}-\d{2} 23:59:59$/.test(form.endDate || '')) return json({ code: 500, msg: 'Expected full date-time range' });
    const triggerDayList = Array.from({ length: 14 }, (_, i) => { const d = new Date(Date.now() - (13 - i) * 86400000); return d.toISOString().slice(0, 10); });
    return json({ code: 200, content: { triggerDayList, triggerDayCountRunningList: triggerDayList.map((_, i) => i + 4), triggerDayCountSucList: triggerDayList.map((_, i) => i + 2), triggerDayCountFailList: triggerDayList.map(() => 2) } });
  }
  if (route === '/jobinfo/nextTriggerTime') return json({ code: 200, content: ['2026-09-20 12:00:00','2026-09-20 12:05:00','2026-09-20 12:10:00','2026-09-20 12:15:00','2026-09-20 12:20:00'] });
  if (route === '/user/pageList') return username === 'admin' ? page(users.filter(u => (!form.username || u.username.includes(form.username)) && (Number(form.role) < 0 || u.role === Number(form.role)))) : json({ code: 500, msg: 'No permission' });
  if (route.startsWith('/user/') && username !== 'admin') return json({ code: 500, msg: 'No permission' });
  if (route === '/user/add') { users.push({ id: Math.max(...users.map(u => u.id)) + 1, username: form.username, role: Number(form.role), permission: form.permission, password: null }); return json({ code: 200, content: null }); }
  if (route === '/user/update') { users = users.map(u => u.id === Number(form.id) ? { ...u, username: form.username, role: Number(form.role), permission: form.permission } : u); return json({ code: 200, content: null }); }
  if (route === '/user/remove') { users = users.filter(u => u.id !== Number(form.id)); return json({ code: 200, content: null }); }
  if (route === '/joblog/getJobsByGroup') return json({ code: 200, content: jobs.filter(j => j.jobGroup === Number(form.jobGroup) && authorized(j.jobGroup)) });
  if (route === '/jobinfo/pageList') return page(jobs.filter(j => authorized(j.jobGroup) && (Number(form.jobGroup) < 1 || j.jobGroup === Number(form.jobGroup)) && (Number(form.triggerStatus) === -1 || j.triggerStatus === Number(form.triggerStatus)) && j.jobDesc.includes(form.jobDesc || '') && j.executorHandler.includes(form.executorHandler || '') && j.author.includes(form.author || '')));
  if (route === '/joblog/pageList') {
    const rows = jobs.filter(j => authorized(j.jobGroup) && (!Number(form.jobGroup) || j.jobGroup === Number(form.jobGroup)) && (!Number(form.jobId) || j.id === Number(form.jobId))).map(j => ({ id: j.id + 400, jobId: j.id, jobGroup: j.jobGroup, triggerTime: 1789603200000, handleTime: 1789603201000, triggerCode: 200, handleCode: 200, triggerMsg: 'Demo dispatched', handleMsg: 'Demo completed', executorAddress: 'http://demo-node.invalid:9999/', executorHandler: j.executorHandler }));
    return page(rows.filter(() => ![2, 3].includes(Number(form.logStatus))));
  }
  if (route === '/joblog/logDetailCat') return json({ code: 200, content: { fromLineNum: Number(form.fromLineNum), toLineNum: Number(form.fromLineNum) + 1, logContent: '[demo] starting job\n[demo] completed\n', end: true } });
  const values = Object.fromEntries(Object.entries(form).map(([k, v]) => [k, numeric.includes(k) ? Number(v) : v]));
  const id = Number(form.id), existing = jobs.find(j => j.id === id);
  if (route.startsWith('/jobgroup/') && username !== 'admin') return json({ code: 500, msg: 'No permission' });
  if (route.startsWith('/jobinfo/') && ((existing && !authorized(existing.jobGroup)) || (form.jobGroup && !authorized(Number(form.jobGroup))))) return json({ code: 500, msg: 'No permission' });
  if (['/jobinfo/add', '/jobinfo/update'].includes(route) && (!['NONE', 'CRON', 'FIX_RATE'].includes(form.scheduleType) || (form.scheduleType === 'CRON' && form.scheduleConf === 'invalid'))) return json({ code: 500, msg: 'Invalid schedule (local fixture)' });
  if (route === '/jobinfo/add') jobs.push({ ...values, id: Math.max(100, ...jobs.map(j => j.id)) + 1, triggerStatus: 0 });
  else if (route === '/jobinfo/update') jobs = jobs.map(j => j.id === id ? { ...j, ...values } : j);
  else if (route === '/jobinfo/remove') jobs = jobs.filter(j => j.id !== id);
  else if (route === '/jobinfo/start' || route === '/jobinfo/stop') jobs = jobs.map(j => j.id === id ? { ...j, triggerStatus: route.endsWith('start') ? 1 : 0 } : j);
  else if (route === '/jobinfo/trigger') { /* The fixture records the exact parameter without executing. */ }
  else if (route === '/jobgroup/save') groups.push({ ...values, id: Math.max(0, ...groups.map(g => g.id)) + 1 });
  else if (route === '/jobgroup/update') groups = groups.map(g => g.id === id ? { ...g, ...values } : g);
  else if (route === '/jobgroup/remove') groups = groups.filter(g => g.id !== id);
  else { res.statusCode = 404; return json({ code: 500, msg: 'Unknown fixture route' }); }
  return json({ code: 200, msg: null, content: null });
});
server.listen(Number(process.env.FIXTURE_PORT || 5192), '127.0.0.1', () => console.log(`XXL-JOB fixture: http://127.0.0.1:${server.address().port}${prefix}`));
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => server.close());
