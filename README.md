# XXL-JOB Console for DBX

DBX workbench for official XXL-JOB Admin. Provides separate `2.3.x` and `3.4.x` API profiles; choose the Admin version when creating a connection. These profiles were checked against official `2.3.0` and `3.4.2` source respectively. The latest official release checked for this project is `3.4.2` (June 19, 2026). Other releases and customized distributions are not assumed compatible without testing.

This community plugin is published by **Ezreal-byte** as `io.dbx.xxljob-console`. It does not require direct database access or modify the XXL-JOB server.

## Credits and changes

Many thanks to **[caichangqing1120](https://github.com/caichangqing1120)** for the original [dbx-plugin-xxl-job](https://github.com/caichangqing1120/dbx-plugin-xxl-job), which this project builds on. The original repository and its license remain acknowledged here and in the source history.

This version reorganizes the workbench into a 40 px top navigation with reports, jobs, logs, executors, and admin-only users. It adds a themed ECharts dashboard, seven-field Cron editor with server-calculated run times, sectioned job forms, reusable styled selects, translated interface strings, and live DBX theme and font updates. It also adapts the Admin 2.3.x and 3.4.x report and user APIs, provides a dedicated live log page and new-tab action, and strengthens permission checks for log access. The plugin icon has been replaced with the user-provided SVG.

## Connect

Requires DBX `0.6.14` or newer, Host API `1`. Enter the Admin IP/domain, port, username and password. New connections select `3.4.x` and `/` by default, matching the official 3.4.2 configuration. For stock `2.3.x`, select that version and change the application path to `/xxl-job-admin`. If the Admin is deployed at a custom path, enter that path instead. HTTPS and DBX SSH/proxy/HTTP tunnels are supported. HTTP transmits the login password in clear text: use it only on a trusted private network.

The plugin authenticates to the selected Admin version and keeps the session cookie in memory. Redirects are never followed; expired sessions require reconnecting. Existing connections without a version selector retain the legacy `2.3.x` profile. For a new connection, the default is `3.4.x`.

- View the scheduling report and date trend; filter and paginate jobs, logs, executors and users.
- Create/update BEAN jobs, start/stop/trigger/delete a job after confirmation.
- Admin users can manage executor groups and user accounts. Non-admin users see only their authorized groups and do not see the user menu; writes are checked again by the backend.
- Read running execution logs in a full workbench page with three-second cursor polling. The **New window** action uses DBX's `host.workbench` permission and requires the matching DBX host bridge change for distinct simultaneous log tabs.
- Read-only connections prevent all changes. Ambiguous write results are not retried.

The version profiles differ: `2.3.x` uses `/login`, legacy flat pagination and singular `id`; `3.4.x` uses `/auth/doLogin`, `Response<PageModel>` and `ids[]` for some writes. The plugin never probes a write endpoint to infer a version. An unsupported version is rejected before login. Version selection is not a guarantee for forks or untested deployments.

## Build and verify

Requires Node.js 22+, Go 1.22+ and `@dbx-app/plugin-cli@0.1.9`.

```sh
npm ci --ignore-scripts
npm test
npm run build
go -C backend test -race ./...
go -C backend vet ./...
npm install --global @dbx-app/plugin-cli@0.1.9
npm run package:all
go run scripts/verify-package.go dist/io.dbx.xxljob-console-0.3.0-darwin-arm64.dbxp --handshake
```

`package:all` builds independent native sidecars for macOS, Windows and Linux, both ARM64 and x64. It emits six unsigned `.dbxp` candidates, their `.artifact.json` metadata, `SHA256SUMS-v0.3.0.txt` and `release-candidates.json`. Cross-platform packages are architecture and checksum checked, not claimed as tested in a DBX desktop on every OS. These unsigned packages are review candidates: normal marketplace installation requires DBX Store review and signing.

Local fixture testing uses synthetic data only (`node scripts/fixture-server.mjs`); no real Admin instance or production task was used. Do not enter a real password into the development host because `.dbx-dev` stores its fixture credentials in plaintext.

## Marketplace

Source and unsigned candidates are published to the author's GitHub Release. A separate candidate PR to [`t8y2/dbx-store`](https://github.com/t8y2/dbx-store) is required; DBX maintainers review and sign the exact candidate bytes before the plugin can appear in the official catalog. Creating a public repository or Release does not itself list or sign the plugin. No signing private key belongs in this repository.

The third-party notices in `assets/` and the vendored DBX Go SDK license in `backend/sdk/` must accompany every package. The plugin's own source is MIT-licensed; see `LICENSE`.
