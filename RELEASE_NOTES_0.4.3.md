# XXL-JOB Console 0.4.3 · 2026-11-13

## 修复外部 MCP 工具不可见

- 解决 [issue #7](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/issues/7)：DBX 新版本要求 MCP 贡献在 `manifest.json` 中声明每个工具的 `access`，否则全部显示为「作者未声明」且不会开放。
- 在 `io.dbx.xxljob-console.mcp` 中补充 5 个 toolset 与 22 个工具的访问级别：
  - 查询诊断 12 个 → `read`
  - 调度操作 / 任务配置 / 执行器创建更新 → `write`
  - 删除任务、删除执行器 → `full`
- 后端工具目录与按连接授权逻辑不变；本次仅补齐宿主侧声明，使外部 MCP 与插件中心授权面板可正常开放工具。
- 新增 manifest 与后端工具目录的一致性测试，后续新增或调整工具时不会再遗漏上架声明。

## 执行器下拉框支持模糊搜索

- 解决 [issue #8](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/issues/8)：任务/日志筛选与新增任务弹窗的执行器下拉框均支持输入关键字过滤。
- 支持按执行器名称、AppName 和 ID 搜索，不区分大小写；多个关键字以空格分隔时按 AND 匹配。
- 支持方向键、Home/End、Enter 和 Escape，打开下拉框后可直接输入，无需先用鼠标点击搜索框。
- 新增任务时默认带入任务管理页当前选中的执行器；选择「全部执行器」时仍使用首个可用执行器。
- 面向 1000+ 执行器的部署优化，保留原有分页与权限过滤逻辑。

## 使用方法

升级到 `0.4.3` 后，打开插件中心 → 本插件 → MCP 授权，确认工具不再是「未声明」。再在连接的 **AI/MCP** 面板开启查询工具，并在 DBX MCP 设置中授权对应连接。

最低 DBX 0.6.43、Host API ^1.4.0。在已收紧声明校验的 DBX（如 0.6.64+）上，MCP 声明修复是外部 MCP 可见的必要条件。
