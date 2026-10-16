export type Context = { connectionId?: string; page?: string; tabTitle?: string; log?: Record<string, unknown> };
export type AiContext = { connectionId: string; page: string; resource?: { kind: string; id: number; executorId?: number; jobId?: number; triggerTime?: string | number } };
declare global {
  interface Window {
    dbxPlugin?: {
      ready: Promise<unknown>; context: Context; locale?: string;
      invoke<T>(method: string, params: Record<string, unknown>, options?: { timeoutMs: number }): Promise<T>;
      onContext(fn: (context: Context) => void): () => void;
      openWorkbench?(contributionId: string, context?: Context, options?: { forceNew?: boolean }): Promise<void>;
      ai?: {
        openConversation(options: { title: string; prompt: string; context: AiContext; send?: boolean; mode: 'agent' }): Promise<void>;
        setRecommendations(update: { context: AiContext; items: { id: string; label: string; prompt: string; order: number }[] }): Promise<void>;
        clearRecommendations(): Promise<void>;
      };
    };
  }
}
export async function invoke<T>(connectionId: string, method: string, params: Record<string, unknown> = {}): Promise<T> {
  if (!window.dbxPlugin || !connectionId) throw new Error('请从 DBX 连接打开工作台');
  return window.dbxPlugin.invoke<T>(method, { ...params, connectionId }, { timeoutMs: 45000 });
}
