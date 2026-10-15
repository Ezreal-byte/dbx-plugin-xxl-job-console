# XXL-JOB Console 0.3.3 · 2026-10-15

## 修复与改进

- 合并 [PR #3](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/pull/3)：修复 Admin 3.1.0 执行器列表返回 HTTP 400。查询现在显式发送空的 `appname`、`title`，满足必填参数绑定，并保持无筛选语义。
- 增加两种现有 API 配置的回归测试，检查筛选参数确实存在于 POST 表单中且保持空值；保留普通用户的执行器授权范围及只读、写操作确认检查。
- 修复 DBX Store 自动扫描所需的 `release-candidates.json` 格式，使用包内 manifest 的身份和权限生成 `plugin + artifacts` 元数据。
- 增加 Windows/Linux CI：前端测试与构建、Go race 测试、静态检查、SDK 测试和本机 `.dbxp` 协议握手验证。

## 兼容范围

- Admin 3.1.0 可选择 `2.3.x` API 配置；本次修复针对执行器查询，尚未完成该版本全部操作的实机验证。
- 官方 3.2.0 的执行器查询也要求这两个参数，但其认证已引入 XXL-SSO；本次查询修复不代表插件已完整支持官方 3.2.0。
- 现有 `2.3.x`、`3.4.x` 配置保持原有行为，最低 DBX 版本仍为 `0.6.14`。

## 安装包

提供 macOS、Windows、Linux 的 ARM64 和 x64 共六个平台 `.dbxp` 候选包。Release 附件用于 DBX Store 审核；商店安装版需由维护者签名并合并更新。

感谢 [loong95](https://github.com/loong95) 提交修复，也感谢 [caichangqing1120](https://github.com/caichangqing1120/dbx-plugin-xxl-job) 的原始插件。
