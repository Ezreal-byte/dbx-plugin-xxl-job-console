import { spawn } from 'node:child_process';
import { mkdirSync, openSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const output = resolve(root, 'output/preview');
mkdirSync(output, { recursive: true });
const cli = process.env.DBX_PLUGIN_CLI;
if (!cli) throw new Error('Set DBX_PLUGIN_CLI to the official CLI executable');
const children = [
  ['fixture', process.execPath, ['scripts/fixture-server.mjs']],
  ['host', cli, ['dev', '--path', root, '--port', '0', '--data-dir', resolve(root, '.dbx-dev')]],
].map(([name, command, args]) => {
  const fd = openSync(resolve(output, `${name}.log`), 'a');
  const child = spawn(command, args, { cwd: root, detached: true, stdio: ['ignore', fd, fd], env: { ...process.env, FIXTURE_PORT: process.env.FIXTURE_PORT || '0' } });
  child.unref();
  return { name, pid: child.pid, log: resolve(output, `${name}.log`) };
});
writeFileSync(resolve(output, 'processes.json'), JSON.stringify(children, null, 2));
console.log(JSON.stringify(children));
