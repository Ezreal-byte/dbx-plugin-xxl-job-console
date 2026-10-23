# Admin 2.2.0 compatibility verification

Date: 2026-10-23. Plugin: 0.4.2. Request: [issue #6](https://github.com/Ezreal-byte/dbx-plugin-xxl-job-console/issues/6).

## Contract evidence

Official XXL-JOB `2.2.0` source pinned to [`1874c4854f81d6af4a9471d3fe111cdbf713f336`](https://github.com/xuxueli/xxl-job/tree/1874c4854f81d6af4a9471d3fe111cdbf713f336).

- `JobInfoController` / `XxlJobInfo`: legacy `jobCron`, singular IDs, `/jobinfo/add`, `/jobinfo/nextTriggerTime` with `cron`.
- `XxlJobServiceImpl`: Cron required for create/update, no schedule-type or configurable misfire fields.
- `JobScheduleHelper`: lateness over five seconds is skipped; smaller lateness may trigger once. The normalized `DO_NOTHING` value is only a compatibility representation; it is never sent as a 2.2 setting.
- `IndexController`, `LoginService`, `JobGroupController`, `JobLogController`: legacy login cookie, flat pagination, overview, executor and log contracts.

## Automated coverage

- `npm test`: 18 tests; `npm run build`: TypeScript and production build.
- `go -C backend test -race ./...`, `go -C backend vet ./...`, `go -C backend/sdk test ./...`.
- All four profiles exercise public UI/MCP write previews, invalid/replayed tokens, read-only rejection, patch preservation and copy behavior. Existing 3.5 token preservation coverage remains.
- Dedicated 2.2 tests cover default-deny without login, job normalization, malformed Cron responses, Cron preview parameters, unsupported scheduling rejection in both UI and MCP, create wire fields, overview, bounded log snapshots, job options and login context paths.
- Frontend checks cover version selection/localizations, Cron-only choices and unsupported form rejection.

## Real Admin verification

Disposable Docker network `dbx-xxl22-issue6`, containers `dbx-xxl22-admin` and `dbx-xxl22-mysql`. Only Admin is exposed, on `127.0.0.1:18022`; MySQL has no published port. The official 2.2 SQL initializes a fresh database.

Admin image: `xuxueli/xxl-job-admin:2.2.0`, digest `sha256:bf9043ac702df0c6042e69f22ad6da62cf5e73dc065362e48217236c600ea916`. Database: `mysql:5.7.34`.

Opt-in `TestLiveAdmin22` uses the plugin's public workbench RPC and `mcp/call` implementation, with a temporary plugin data directory. It grants only this isolated connection and cleans up its own jobs/executor through previewed deletion.

Passed:

1. Default MCP denial, login, connection info, overview/report, server Cron preview.
2. Workbench executor creation; MCP task creation with a future Cron; detail round trip confirms stopped scheduling.
3. Workbench partial edit changes the Cron without clearing saved parameters.
4. MCP start/stop verified by fresh server reads; copy verified stopped.
5. Manual trigger accepted; Admin produces a dispatch failure because this executor deliberately has no nodes. This verifies acceptance and log lookup, not successful business execution.
6. MCP details/log list; cleanup removes both temporary jobs before deleting their executor.

### Reproduction

Use a disposable official 2.2.0 deployment with its matching SQL, not a daily-use server. The test requires a loopback URL and explicit password. PowerShell:

```powershell
$env:XXLJOB22_TEST_URL = 'http://127.0.0.1:18022/xxl-job-admin'
$env:XXLJOB22_TEST_PASSWORD = '<isolated Admin password>'
go -C backend test -run '^TestLiveAdmin22$' -v -count=1
Remove-Item Env:XXLJOB22_TEST_URL, Env:XXLJOB22_TEST_PASSWORD
```

The live test is skipped in normal tests/CI unless explicitly configured. The first local run caught a typo in the test RPC name (`jobDetail` instead of `jobDetails`); cleanup succeeded. The corrected full integration run passed.

## Boundaries

- No real executor node: log-body forwarding is covered by contract fixtures, not a live business executor.
- No new desktop/model-session end-to-end claim. Real Admin calls use the same workbench/MCP backend entry points; discovery, account permission and confirmation behavior retain existing regression coverage.
- 2.3/3.4/3.5 regression uses fixtures in this change; previous real-server evidence is recorded separately.
- Release workflow builds six packages; native Windows packaging/protocol checks and remote CI are release gates. Cross-compilation alone does not imply all six platforms were manually tested in DBX.
