# XXL-JOB Console 0.4.1 · 2026-10-21

## Admin 3.5.0 兼容

- 合入 [PR #5](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/pull/5)，解决 [issue #4](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/issues/4)：新增明确的 `3.5.x` 连接选项。
- 适配执行器 `title ↔ name`、任务 `jobDesc ↔ name`，覆盖查询筛选、列表、任务选项及新增和修改；工作台、内置 AI 和外部 MCP 继续使用原有工具字段。
- 修复新版日志快照错误解析旧版触发时间的问题，支持现代接口返回的格式化时间。

## 令牌保护与回归

- 编辑执行器时从后端最新查询保留原有 `accessToken`，令牌不进入列表、详情、预览或 AI/MCP 输出。令牌配置仍通过官方 Admin 页面完成。
- 服务端响应缺少令牌字段或类型异常时拒绝更新，避免误清空已有令牌；列表查询仍可使用。
- UI/MCP 公开入口回归覆盖 2.3、3.4、3.5 的预览确认、无效令牌和只读拒绝，以及预览后令牌轮换的保留行为。

## 升级说明

- **连接 Admin 3.5.0 时，请手动选择 `3.5.x`，应用路径通常为 `/`。** 新连接仍默认 `3.4.x`，历史连接的 `2.3` 回退行为保留。
- 延续 0.4.0 的 22 个内置 AI/MCP 工具、按连接授权和危险操作预览确认。最低要求仍为 DBX `0.6.43`、Host API `^1.4.0`。
- PR 作者已记录 macOS / DBX 上的读取及临时资源增改删实测；真实调度、用户写入和非空令牌配置的实机验证范围见 [验证记录](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/blob/v0.4.1/docs/VERIFICATION_3.5.0.md)。
- 提供 macOS、Windows、Linux 的 ARM64 和 x64 共六个平台未签名 `.dbxp` 候选包；商店版需经 DBX Store 审核签名。

感谢 [@zzhao5726](https://github.com/zzhao5726) 提交问题、兼容适配、回归测试和实机验证记录！
