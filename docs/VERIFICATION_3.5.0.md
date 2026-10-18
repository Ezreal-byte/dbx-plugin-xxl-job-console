# XXL-JOB Admin 3.5.0 兼容验证

关联 [Issue #4](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/issues/4)。基于插件 0.4.0 / `042ff907`。

## 方案

- 新增独立 `3.5.x` 连接选项，保留 `2.3.x`、`3.4.x`、新连接默认 `3.4` 和历史连接回退 `2.3`。
- 在请求和分页响应边界转换执行器 `title ↔ name`、任务 `jobDesc ↔ name`。工作台和 AI/MCP 继续使用原有字段；覆盖筛选、任务选项、创建和更新。
- 3.5.0 使用新版登录、分页、概览、日志和批量 ID 协议。日志正文仅在 2.3 分支解析旧版触发时间，避免现代版本的格式化时间被本地校验拦截。
- 更新执行器时从最新查询保留 `accessToken`，仅用于后端回传；执行器列表、详情快照和预览不增加令牌字段。令牌配置继续使用官方 Admin 页面。

## 官方契约依据

对照官方 `3.5.0` 标签的 [JobGroupController](https://github.com/xuxueli/xxl-job/blob/3.5.0/xxl-job-admin/src/main/java/com/xxl/job/admin/business/controller/JobGroupController.java)、[JobInfoController](https://github.com/xuxueli/xxl-job/blob/3.5.0/xxl-job-admin/src/main/java/com/xxl/job/admin/business/controller/JobInfoController.java)、[JobLogController](https://github.com/xuxueli/xxl-job/blob/3.5.0/xxl-job-admin/src/main/java/com/xxl/job/admin/business/controller/JobLogController.java) 和 [XxlJobGroupMapper](https://github.com/xuxueli/xxl-job/blob/3.5.0/xxl-job-admin/src/main/resources/mapper/business/XxlJobGroupMapper.xml)。任务分页 `name` 为必传参数，执行器更新会写入 `access_token`。

## 已完成

本机 macOS ARM64，Node.js 24、Go 1.27.2。使用仓库现有测试文件扩展兼容回归，未新增测试文件。

- `npm ci --ignore-scripts`
- `npm test`：17 项通过。
- `npm run build`：类型检查和构建通过。
- `go -C backend test -race ./...`：完整后端回归及并发检查通过。
- `go -C backend vet ./...`：通过。
- `go -C backend/sdk test ./...`：通过。
- `node scripts/package.mjs darwin-arm64`：打包通过；架构、入口、包内容、SHA256 和元数据校验通过。
- `git diff --check`：通过。

兼容回归覆盖 3.4/3.5 登录、默认和自定义路径、执行器列表、任务分页/选项、任务和执行器新增/更新、原令牌保留、调度操作、日志正文、概览和报表。2.3 原有测试以及三版本筛选字段/用户请求适配均通过。

## DBX 桌面实机验证（2026-10-18）

macOS ARM64、DBX 0.6.47，通过插件中心安装本地开发包 `0.4.1-local.3.5.0.2`，使用现有官方 Admin 3.5.0 实例，连接版本选为 `3.5.x`。

- 登录及运行报表正常：3 个任务、1 个执行器、3 条历史调度记录。
- 执行器名称、AppName、自动注册节点正常，不再出现“执行器字段缺失”。
- 任务名称、名称筛选、详情与近期记录正常；调度日志的执行器及任务下拉名称正确，任务筛选正常。
- 成功及失败历史记录的日志正文均可读取；成功记录轮询至“日志读取完成”。
- 首次桌面验证发现 `logSnapshot` 在调用现代日志接口前仍解析旧版时间格式；修复为只在 2.3 分支解析，并在现有 3.4/3.5 契约测试中覆盖完整快照调用。重新运行后端 race 测试及 vet，通过；重新打包、校验架构/入口/内容/SHA256/元数据及后端握手后安装复验。

## 验收边界

- 桌面实机验证只执行读取，未修改真实 Admin 任务、执行器或用户，也未启动、停止或触发任务。
- 未完成真实 Admin 3.5.0 的桌面写操作验收；跨平台 CI 结果以 PR 上的 GitHub Actions 为准。
- 未签名候选包不代表商店已审核或发布；用户安装新版后，连接到 3.5.0 时需手动选择 `3.5.x`。
