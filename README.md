# XXL-JOB Console for DBX

DBX workbench for official XXL-JOB Admin. Provides separate `2.3.x` and `3.4.x` API profiles; choose the Admin version when creating a connection. These profiles were checked against official `2.3.0` and `3.4.2` source respectively. The latest official release checked for this project is `3.4.2` (June 19, 2026). Other releases and customized distributions are not assumed compatible without testing.

This community plugin is published by **Ezreal-byte** (catalog publisher ID `ezreal-byte`) as `io.dbx.xxljob-console`. It does not require direct database access or modify the XXL-JOB server.

## Credits and changes

Many thanks to **[caichangqing1120](https://github.com/caichangqing1120)** for the original [dbx-plugin-xxl-job](https://github.com/caichangqing1120/dbx-plugin-xxl-job), which this project builds on. The original repository and its license remain acknowledged here and in the source history.

This version reorganizes the workbench into a 40 px top navigation with reports, jobs, logs, executors, and admin-only users. It adds a themed ECharts dashboard, seven-field Cron editor with server-calculated run times, sectioned job forms, reusable styled selects, translated interface strings, and live DBX theme and font updates. Dense bordered tables have labeled actions; common job actions are inline and registration nodes, edit, copy, and delete sit under More. It also adapts the Admin 2.3.x and 3.4.x report and user APIs, provides a dedicated live log page and new-tab action, and strengthens permission checks for log access. The plugin icon has been replaced with the user-provided SVG.

Version `0.3.4` re-queries a log in each new workbench tab before reading it, retries a verified lookup if the shared log cache is replaced, and follows appended output to the bottom by default. Manual runs load the current configured job parameters before opening the execution form.

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

Requires Node.js 22+ and Go 1.22+.

```sh
npm ci --ignore-scripts
npm test
npm run build
go -C backend test -race ./...
go -C backend vet ./...
npm run package:all
go run scripts/verify-package.go dist/io.dbx.xxljob-console-0.3.4-windows-x64.dbxp --handshake
```

`package:all` builds independent native sidecars for macOS, Windows and Linux, both ARM64 and x64. It emits six unsigned `.dbxp` candidates, their `.artifact.json` metadata, a versioned `SHA256SUMS` file and `release-candidates.json`. Cross-platform packages are architecture and checksum checked, not claimed as tested in a DBX desktop on every OS. These unsigned packages are review candidates: normal marketplace installation requires DBX Store review and signing.

Local fixture testing uses synthetic data (`node scripts/fixture-server.mjs`). A separate [10-minute live-log executor](demo/live-log-executor/README.md) can be used with a local XXL-JOB Admin 2.3.0 instance to check polling; it is not included in the `.dbxp` package. Do not enter a real password into the development host because `.dbx-dev` stores its fixture credentials in plaintext.

## Marketplace

The store currently lists version `0.3.1`. For an upgrade, publish a versioned source tag and GitHub Release after review. The release workflow builds six unsigned candidates and `release-candidates.json`; submit that new version in `candidates/io.dbx.xxljob-console.json` to [`t8y2/dbx-store`](https://github.com/t8y2/dbx-store). Store maintainers review, sign and merge the candidate. Do not edit the signed catalog entry directly. The dynamic `#ID 日志查看` tab title also needs the companion DBX host change in `PluginWorkbenchTab.vue`; hosts without it retain the manifest's generic log-tab label. No signing private key belongs in this repository.

The third-party notices in `assets/` and the vendored DBX Go SDK license in `backend/sdk/` must accompany every package. The plugin's own source is MIT-licensed; see `LICENSE`.
