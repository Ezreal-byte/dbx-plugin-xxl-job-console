export type Context = { connectionId?: string; page?: string; tabTitle?: string; log?: Record<string, unknown> };
declare global {
  interface Window {
    dbxPlugin?: {
      ready: Promise<unknown>; context: Context; locale?: string;
      invoke<T>(method: string, params: Record<string, unknown>, options?: { timeoutMs: number }): Promise<T>;
      onContext(fn: (context: Context) => void): () => void;
      openWorkbench?(contributionId: string, context?: Context, options?: { forceNew?: boolean }): Promise<void>;
    };
  }
}
export async function invoke<T>(connectionId: string, method: string, params: Record<string, unknown> = {}): Promise<T> {
  if (!window.dbxPlugin || !connectionId) throw new Error('请从 DBX 连接打开工作台');
  return window.dbxPlugin.invoke<T>(method, { ...params, connectionId }, { timeoutMs: 25000 });
}
