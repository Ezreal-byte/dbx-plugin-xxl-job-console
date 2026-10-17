# XXL-JOB Admin 3.5.0 兼容验证

关联 [Issue #4](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/issues/4)。基于插件 0.4.0 / `042ff907`。

## 方案

- 新增独立 `3.5.x` 连接选项，保留 `2.3.x`、`3.4.x`、新连接默认 `3.4` 和历史连接回退 `2.3`。
- 在请求和分页响应边界转换执行器 `title ↔ name`、任务 `jobDesc ↔ name`。工作台和 AI/MCP 继续使用原有字段；覆盖筛选、任务选项、创建和更新。
- 3.5.0 使用新版登录、分页、概览、日志和批量 ID 协议。补齐日志正文校验，避免误要求旧版执行器地址和触发时间。
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

## 验收边界

- 本次执行的是合成 HTTP 契约测试和构建校验，未安装到 DBX 桌面，也未修改真实 Admin 任务、执行器或用户。
- 未完成真实 Admin 3.5.0 的桌面写操作验收；跨平台 CI 结果以 PR 上的 GitHub Actions 为准。
- 未签名候选包不代表商店已审核或发布；用户安装新版后，连接到 3.5.0 时需手动选择 `3.5.x`。
