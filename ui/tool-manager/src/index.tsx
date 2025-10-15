import React, {
  useCallback,
  useEffect,
  useMemo,
  useState,
  useSyncExternalStore,
  type SetStateAction,
} from "react";
import { createRoot } from "react-dom/client";

type ToolName = "mcp-find" | "mcp-add" | "mcp-remove";
type WidgetStatus = "success" | "error" | "info";

type ToolContent = {
  type?: string;
  text?: string;
  [key: string]: unknown;
};

type CallToolResponse = {
  content?: ToolContent[];
  structuredContent?: unknown;
  isError?: boolean;
  _meta?: Record<string, unknown>;
};

type ToolManagerAction = {
  type: "find" | "add" | "remove";
  status: "success" | "error";
  message?: string;
  server?: string;
};

type ToolManagerServer = {
  name: string;
  description?: string;
  type?: string;
  remoteUrl?: string;
  image?: string;
  longLived?: boolean;
  requiredSecrets?: string[];
  configSchema?: unknown;
  tools?: { name: string; description?: string }[];
  oauthProviders?: string[];
  isActive?: boolean;
};

type ToolManagerState = {
  view?: string;
  sourceTool?: string;
  status?: WidgetStatus;
  message?: string;
  query?: string;
  limit?: number;
  totalMatches?: number;
  activeServers: string[];
  results?: ToolManagerServer[];
  server?: ToolManagerServer;
  lastAction?: ToolManagerAction;
  timestamp?: string;
  error?: string;
};

type OpenAiGlobals = {
  toolOutput?: CallToolResponse | null;
  widgetState?: ToolManagerState | null;
  theme?: "light" | "dark";
  displayMode?: "inline" | "fullscreen" | "pip";
  maxHeight?: number;
  locale?: string;
  userAgent?: { device?: { type?: string } };
  safeArea?: { insets: { top: number; right: number; bottom: number; left: number } };
};

type OpenAiAPI = {
  callTool?: (name: string, args: Record<string, unknown>) => Promise<CallToolResponse>;
  sendFollowUpMessage?: (args: { prompt: string }) => Promise<void>;
  openExternal?: (payload: { href: string }) => void;
  requestDisplayMode?: (args: { mode: "pip" | "inline" | "fullscreen" }) => Promise<{ mode: string }>;
  setWidgetState?: (state: unknown) => Promise<void>;
  setLayout?: (layout: unknown) => Promise<void>;
};

type OpenAiHost = OpenAiAPI & OpenAiGlobals;

declare global {
  interface Window {
    openai?: OpenAiHost;
  }

  interface WindowEventMap {
    [SET_GLOBALS_EVENT_TYPE]: SetGlobalsEvent;
  }
}

type SetGlobalsEvent = CustomEvent<{
  globals: Partial<OpenAiGlobals>;
}>;

const SET_GLOBALS_EVENT_TYPE = "openai:set_globals";
const STYLE_TAG_ID = "mcp-tool-manager-style";
const ROOT_ELEMENT_ID = "mcp-tool-manager-root";
const DEFAULT_LIMIT = 10;

const EMPTY_STATE: ToolManagerState = {
  status: "info",
  activeServers: [],
  message: "Search the catalog to find MCP servers you can enable.",
};

function injectStyles(): void {
  if (typeof document === "undefined") {
    return;
  }
  if (document.getElementById(STYLE_TAG_ID)) {
    return;
  }
  const style = document.createElement("style");
  style.id = STYLE_TAG_ID;
  style.textContent = `
    :root {
      color-scheme: light dark;
      font-family: "Inter", "Segoe UI", system-ui, -apple-system, sans-serif;
    }
    #${ROOT_ELEMENT_ID} {
      height: 100%;
    }
    .mcp-tool-manager {
      display: flex;
      flex-direction: column;
      gap: 1rem;
      height: 100%;
      box-sizing: border-box;
      padding: 1.25rem;
      background: transparent;
      color: var(--mcp-tool-manager-fg, inherit);
    }
    .mcp-tool-manager header h1 {
      margin: 0;
      font-size: 1.25rem;
      font-weight: 600;
    }
    .mcp-tool-manager header p {
      margin: 0.25rem 0 0;
      color: var(--mcp-tool-manager-muted, rgba(120, 120, 120, 0.9));
      font-size: 0.95rem;
      line-height: 1.4;
    }
    .mcp-status {
      padding: 0.75rem 1rem;
      border-radius: 0.75rem;
      font-size: 0.95rem;
      line-height: 1.3;
    }
    .mcp-status.success {
      background: rgba(16, 185, 129, 0.14);
      color: rgba(6, 95, 70, 0.95);
    }
    .mcp-status.error {
      background: rgba(248, 113, 113, 0.18);
      color: rgba(153, 27, 27, 0.95);
    }
    .mcp-status.info {
      background: rgba(59, 130, 246, 0.16);
      color: rgba(30, 64, 175, 0.95);
    }
    .mcp-search-card {
      border: 1px solid rgba(120, 120, 120, 0.2);
      border-radius: 1rem;
      padding: 1rem;
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
      background: color-mix(in srgb, currentColor 4%, transparent);
      box-shadow: 0 2px 8px rgba(15, 15, 15, 0.04);
    }
    .mcp-search-card form {
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
    }
    .mcp-search-row {
      display: flex;
      gap: 0.5rem;
      flex-wrap: wrap;
    }
    .mcp-search-row input[type="text"] {
      flex: 1 1 220px;
      padding: 0.55rem 0.75rem;
      border-radius: 0.65rem;
      border: 1px solid rgba(120, 120, 120, 0.35);
      font-size: 0.95rem;
      background: rgba(255, 255, 255, 0.04);
      color: inherit;
    }
    .mcp-search-row button {
      padding: 0.55rem 1rem;
      border-radius: 0.65rem;
      border: none;
      font-weight: 600;
      font-size: 0.95rem;
      background: rgba(59, 130, 246, 0.92);
      color: #fff;
      cursor: pointer;
    }
    .mcp-search-row button[disabled],
    .mcp-action-button[disabled] {
      opacity: 0.55;
      cursor: not-allowed;
    }
    .mcp-active-list {
      display: flex;
      flex-wrap: wrap;
      gap: 0.4rem;
    }
    .mcp-chip {
      padding: 0.35rem 0.65rem;
      border-radius: 999px;
      font-size: 0.85rem;
      background: rgba(59, 130, 246, 0.12);
      color: rgba(37, 99, 235, 0.95);
    }
    .mcp-card-list {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 1rem;
    }
    .mcp-card {
      border: 1px solid rgba(120, 120, 120, 0.18);
      border-radius: 1rem;
      padding: 1rem;
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
      background: color-mix(in srgb, currentColor 3%, transparent);
      box-shadow: 0 2px 6px rgba(15, 15, 15, 0.04);
    }
    .mcp-card h2 {
      margin: 0;
      font-size: 1.05rem;
      font-weight: 600;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 0.5rem;
    }
    .mcp-badge {
      font-size: 0.75rem;
      font-weight: 600;
      padding: 0.2rem 0.45rem;
      border-radius: 999px;
      background: rgba(16, 185, 129, 0.18);
      color: rgba(6, 95, 70, 0.9);
    }
    .mcp-card p {
      margin: 0;
      font-size: 0.9rem;
      line-height: 1.45;
      color: var(--mcp-tool-manager-muted, rgba(120, 120, 120, 0.9));
    }
    .mcp-card dl {
      margin: 0;
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 0.35rem 0.75rem;
      font-size: 0.85rem;
    }
    .mcp-card dt {
      font-weight: 600;
      color: rgba(100, 100, 100, 0.9);
    }
    .mcp-card dd {
      margin: 0;
      color: inherit;
    }
    .mcp-card .tool-list {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
      margin-top: 0.25rem;
    }
    .mcp-card .tool-list span {
      font-size: 0.8rem;
      color: rgba(120, 120, 120, 0.9);
    }
    .mcp-card-actions {
      margin-top: auto;
      display: flex;
      gap: 0.5rem;
      flex-wrap: wrap;
    }
    .mcp-action-button {
      flex: 0 0 auto;
      border-radius: 0.6rem;
      border: none;
      padding: 0.5rem 0.9rem;
      font-size: 0.9rem;
      font-weight: 600;
      cursor: pointer;
      transition: transform 0.1s ease;
    }
    .mcp-action-button.primary {
      background: rgba(16, 185, 129, 0.9);
      color: white;
    }
    .mcp-action-button.secondary {
      background: rgba(248, 113, 113, 0.9);
      color: white;
    }
    .mcp-empty {
      font-size: 0.9rem;
      color: var(--mcp-tool-manager-muted, rgba(120, 120, 120, 0.9));
      padding: 0.5rem 0;
    }
    .mcp-footer {
      font-size: 0.75rem;
      color: rgba(120, 120, 120, 0.75);
      text-align: right;
    }
  `;
  document.head.appendChild(style);
}

function useOpenAiGlobal<K extends keyof OpenAiGlobals>(key: K): OpenAiGlobals[K] | undefined {
  return useSyncExternalStore(
    (onChange) => {
      const handler = (event: Event): void => {
        const custom = event as SetGlobalsEvent;
        if (custom.detail?.globals && key in custom.detail.globals) {
          onChange();
        }
      };
      window.addEventListener(SET_GLOBALS_EVENT_TYPE, handler, { passive: true });
      return () => window.removeEventListener(SET_GLOBALS_EVENT_TYPE, handler);
    },
    () => window.openai?.[key],
    () => undefined,
  );
}

function useToolOutput(): CallToolResponse | null | undefined {
  return useOpenAiGlobal("toolOutput");
}

function useWidgetState<T>(
  defaultState?: T | (() => T | null) | null,
): readonly [T | null, (state: SetStateAction<T | null>) => void] {
  const widgetStateFromWindow = useOpenAiGlobal("widgetState") as T | null | undefined;
  const [widgetState, setWidgetStateInternal] = useState<T | null>(() => {
    if (widgetStateFromWindow != null) {
      return widgetStateFromWindow;
    }
    if (typeof defaultState === "function") {
      return (defaultState as () => T | null)();
    }
    return defaultState ?? null;
  });

  useEffect(() => {
    if (widgetStateFromWindow !== undefined) {
      setWidgetStateInternal(widgetStateFromWindow);
    }
  }, [widgetStateFromWindow]);

  const setWidgetState = useCallback(
    (value: SetStateAction<T | null>) => {
      setWidgetStateInternal((prev) => {
        const next = typeof value === "function" ? (value as (prevState: T | null) => T | null)(prev) : value;
        if (next != null) {
          void window.openai?.setWidgetState?.(next);
        }
        return next;
      });
    },
    [],
  );

  return [widgetState, setWidgetState] as const;
}

function dedupeStrings(values: string[] | undefined): string[] {
  if (!values || values.length === 0) {
    return [];
  }
  const seen = new Set<string>();
  values.forEach((value) => {
    if (value) {
      seen.add(value);
    }
  });
  return Array.from(seen).sort((a, b) => a.localeCompare(b));
}

function normalizeServer(value: unknown): ToolManagerServer {
  if (!value || typeof value !== "object") {
    return { name: "" };
  }
  const data = value as Record<string, unknown>;
  const toStringArray = (input: unknown): string[] | undefined => {
    if (!Array.isArray(input)) {
      return undefined;
    }
    return input.map((item) => String(item)).filter(Boolean);
  };
  const tools = Array.isArray(data.tools)
    ? data.tools
        .map((tool) => {
          if (!tool || typeof tool !== "object") {
            return null;
          }
          const record = tool as Record<string, unknown>;
          const name = typeof record.name === "string" ? record.name : undefined;
          if (!name) {
            return null;
          }
          return {
            name,
            description: typeof record.description === "string" ? record.description : undefined,
          };
        })
        .filter(Boolean) as { name: string; description?: string }[]
    : undefined;

  return {
    name: typeof data.name === "string" ? data.name : "",
    description: typeof data.description === "string" ? data.description : undefined,
    type: typeof data.type === "string" ? data.type : undefined,
    remoteUrl:
      typeof (data.remoteUrl as string) === "string"
        ? (data.remoteUrl as string)
        : typeof (data.remote_url as string) === "string"
          ? (data.remote_url as string)
          : undefined,
    image: typeof data.image === "string" ? data.image : undefined,
    longLived:
      typeof (data.longLived as boolean) === "boolean"
        ? (data.longLived as boolean)
        : typeof (data.long_lived as boolean) === "boolean"
          ? (data.long_lived as boolean)
          : undefined,
    requiredSecrets:
      toStringArray(data.requiredSecrets) ??
      toStringArray(data.required_secrets) ??
      toStringArray(data.requiredSecretsNames),
    configSchema: data.configSchema ?? data.config_schema,
    tools,
    oauthProviders:
      toStringArray(data.oauthProviders) ?? toStringArray((data.oauth_providers as string[] | undefined)),
  };
}

function normalizeAction(value: unknown): ToolManagerAction | undefined {
  if (!value || typeof value !== "object") {
    return undefined;
  }
  const data = value as Record<string, unknown>;
  const rawType = typeof data.type === "string" ? data.type : "";
  const type: ToolManagerAction["type"] =
    rawType === "add" || rawType === "remove" || rawType === "find" ? rawType : "find";
  const rawStatus = typeof data.status === "string" ? data.status : "";
  const status: ToolManagerAction["status"] = rawStatus === "error" ? "error" : "success";
  return {
    type,
    status,
    message: typeof data.message === "string" ? data.message : undefined,
    server: typeof data.server === "string" ? data.server : undefined,
  };
}

function extractFirstText(response: CallToolResponse | null | undefined): string | undefined {
  if (!response?.content) {
    return undefined;
  }
  for (const item of response.content) {
    if (typeof item?.text === "string" && item.text.trim().length > 0) {
      return item.text.trim();
    }
  }
  return undefined;
}

function parseCallToolResponse(response: CallToolResponse | null | undefined): ToolManagerState | null {
  if (!response) {
    return null;
  }

  const structured = response.structuredContent;
  const base =
    structured && typeof structured === "object" && !Array.isArray(structured)
      ? (structured as Record<string, unknown>)
      : null;

  if (!base) {
    const message = extractFirstText(response) ?? undefined;
    return {
      ...EMPTY_STATE,
      status: response.isError ? "error" : "info",
      message,
    };
  }

  const status = typeof base.status === "string" ? (base.status as WidgetStatus) : undefined;
  const results = Array.isArray(base.results)
    ? (base.results.map((item) => normalizeServer(item)) as ToolManagerServer[])
    : undefined;

  const activeServers = dedupeStrings(
    Array.isArray(base.activeServers)
      ? (base.activeServers as string[])
      : Array.isArray(base.active_servers)
        ? (base.active_servers as string[])
        : undefined,
  );

  const server =
    base.server && typeof base.server === "object" ? normalizeServer(base.server as Record<string, unknown>) : undefined;

  const lastAction = normalizeAction(base.lastAction);

  let message: string | undefined =
    typeof base.message === "string" ? base.message : typeof base.info === "string" ? base.info : undefined;
  if (!message) {
    message = extractFirstText(response);
  }

  return {
    view: typeof base.view === "string" ? base.view : undefined,
    sourceTool: typeof base.sourceTool === "string" ? base.sourceTool : undefined,
    status: status ?? (response.isError ? "error" : undefined),
    message,
    query: typeof base.query === "string" ? base.query : undefined,
    limit: typeof base.limit === "number" ? base.limit : undefined,
    totalMatches: typeof base.totalMatches === "number" ? base.totalMatches : undefined,
    activeServers,
    results,
    server,
    lastAction,
    timestamp: typeof base.timestamp === "string" ? base.timestamp : undefined,
    error: typeof base.error === "string" ? base.error : undefined,
  };
}

function decorateState(state: ToolManagerState | null): ToolManagerState {
  const base = state ?? EMPTY_STATE;
  const activeServers = dedupeStrings(base.activeServers);
  const activeSet = new Set(activeServers);
  const results = (base.results ?? []).map((server) => ({
    ...server,
    isActive: activeSet.has(server.name),
  }));
  const server = base.server ? { ...base.server, isActive: activeSet.has(base.server.name) } : undefined;
  return {
    ...base,
    activeServers,
    results,
    server,
  };
}

type MergeContext = {
  tool: ToolName;
  server?: string;
  message?: string;
  status?: WidgetStatus;
};

function actionForContext(ctx: MergeContext, status: WidgetStatus, message?: string): ToolManagerAction {
  let type: ToolManagerAction["type"] = "find";
  if (ctx.tool === "mcp-add") {
    type = "add";
  } else if (ctx.tool === "mcp-remove") {
    type = "remove";
  }
  return {
    type,
    server: ctx.server,
    status: status === "error" ? "error" : "success",
    message,
  };
}

function mergeState(
  prev: ToolManagerState | null,
  next: ToolManagerState | null,
  ctx: MergeContext,
): ToolManagerState {
  const base = prev ?? EMPTY_STATE;
  if (!next) {
    const status = ctx.status ?? base.status ?? "info";
    const message = ctx.message ?? base.message;
    return decorateState({
      ...base,
      status,
      message,
      lastAction: actionForContext(ctx, status, message),
    });
  }

  const merged: ToolManagerState = {
    ...base,
    ...next,
    activeServers:
      next.activeServers && next.activeServers.length > 0 ? dedupeStrings(next.activeServers) : base.activeServers,
    results: next.results ?? base.results,
    query: next.query ?? base.query,
    limit: next.limit ?? base.limit ?? DEFAULT_LIMIT,
  };

  merged.status = next.status ?? ctx.status ?? base.status ?? "info";
  merged.message = next.message ?? ctx.message ?? base.message;

  if (!next.lastAction) {
    merged.lastAction = actionForContext(ctx, merged.status ?? "info", merged.message);
  }

  return decorateState(merged);
}

function toolOutputSignature(result: CallToolResponse | null | undefined): string {
  if (!result) {
    return "";
  }
  try {
    return JSON.stringify({
      structuredContent: result.structuredContent ?? null,
      isError: Boolean(result.isError),
      message: extractFirstText(result) ?? null,
    });
  } catch {
    return String(Date.now());
  }
}

function formatFallbackMessage(tool: ToolName, ctx: MergeContext, state: ToolManagerState): string {
  if (ctx.message) {
    return ctx.message;
  }
  if (tool === "mcp-find") {
    const term = ctx.message ?? state.query;
    return term ? `Showing results for “${term}”.` : "Showing search results.";
  }
  if (tool === "mcp-add") {
    return ctx.server ? `Added “${ctx.server}”.` : "Server added.";
  }
  if (tool === "mcp-remove") {
    return ctx.server ? `Removed “${ctx.server}”.` : "Server removed.";
  }
  return "Operation completed.";
}

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return "Something went wrong. Please try again.";
}

const App: React.FC = () => {
  const toolOutput = useToolOutput();
  const outputSignature = useMemo(() => toolOutputSignature(toolOutput), [toolOutput]);
  const [state, setState] = useWidgetState<ToolManagerState | null>(() => {
    const parsed = parseCallToolResponse(window.openai?.toolOutput);
    return parsed ?? EMPTY_STATE;
  });

  const decorated = useMemo(() => decorateState(state), [state]);
  const [query, setQuery] = useState(() => decorated.query ?? "");
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [inlineError, setInlineError] = useState<string | null>(null);
  const [isSearching, setIsSearching] = useState(false);

  useEffect(() => {
    if (decorated.query !== undefined) {
      setQuery(decorated.query);
    }
  }, [decorated.query]);

  useEffect(() => {
    if (!toolOutput) {
      return;
    }
    const parsed = parseCallToolResponse(toolOutput);
    const sourceTool =
      parsed?.sourceTool === "mcp-add" || parsed?.sourceTool === "mcp-remove" || parsed?.sourceTool === "mcp-find"
        ? (parsed.sourceTool as ToolName)
        : ("mcp-find" satisfies ToolName);
    const ctx: MergeContext = {
      tool: sourceTool,
      server: parsed?.server?.name ?? parsed?.lastAction?.server,
      message: parsed?.message ?? extractFirstText(toolOutput) ?? undefined,
      status: parsed?.status ?? (toolOutput.isError ? "error" : undefined),
    };
    setState((prev) => mergeState(prev, parsed, ctx));
  }, [setState, toolOutput, outputSignature]);

  const invokeTool = useCallback(
    async (tool: ToolName, args: Record<string, unknown>, ctx: MergeContext) => {
      if (!window.openai?.callTool) {
        throw new Error("Tool calling is unavailable in this context.");
      }
      const response = await window.openai.callTool(tool, args);
      const parsed = parseCallToolResponse(response);
      const status: WidgetStatus =
        parsed?.status ?? (response.isError ? "error" : ctx.status ?? "success");
      const baseState = decorateState(state);
      const message = parsed?.message ?? extractFirstText(response) ?? formatFallbackMessage(tool, ctx, baseState);
      setState((prev) => mergeState(prev, parsed, { ...ctx, tool, message, status }));
      return response;
    },
    [setState, state],
  );

  const handleSearch = useCallback(
    async (event?: React.FormEvent<HTMLFormElement>) => {
      event?.preventDefault();
      const term = query.trim();
      if (!term) {
        setInlineError("Enter a name, description, or keyword to search the catalog.");
        return;
      }
      setInlineError(null);
      setIsSearching(true);
      try {
        const response = await invokeTool(
          "mcp-find",
          { query: term, limit: decorated.limit ?? DEFAULT_LIMIT },
          { tool: "mcp-find", message: `Showing results for “${term}”.`, status: "success" },
        );
        if (response.isError) {
          setInlineError(extractFirstText(response) ?? "The search tool returned an error.");
        }
      } catch (error) {
        setInlineError(formatError(error));
      } finally {
        setIsSearching(false);
      }
    },
    [decorated.limit, invokeTool, query],
  );

  const handleAdd = useCallback(
    async (server: string) => {
      setInlineError(null);
      setPendingAction(`add:${server}`);
      try {
        const response = await invokeTool(
          "mcp-add",
          { name: server },
          { tool: "mcp-add", server, message: `Added “${server}”.`, status: "success" },
        );
        if (response.isError) {
          setInlineError(extractFirstText(response) ?? `Failed to add “${server}”.`);
        }
      } catch (error) {
        setInlineError(formatError(error));
      } finally {
        setPendingAction(null);
      }
    },
    [invokeTool],
  );

  const handleRemove = useCallback(
    async (server: string) => {
      setInlineError(null);
      setPendingAction(`remove:${server}`);
      try {
        const response = await invokeTool(
          "mcp-remove",
          { name: server },
          { tool: "mcp-remove", server, message: `Removed “${server}”.`, status: "info" },
        );
        if (response.isError) {
          setInlineError(extractFirstText(response) ?? `Failed to remove “${server}”.`);
        }
      } catch (error) {
        setInlineError(formatError(error));
      } finally {
        setPendingAction(null);
      }
    },
    [invokeTool],
  );

  const activeServers = decorated.activeServers;
  const results = decorated.results ?? [];
  const statusClass =
    decorated.status === "error" ? "error" : decorated.status === "success" ? "success" : "info";

  return (
    <div className="mcp-tool-manager" role="application">
      <header>
        <h1>MCP Tool Manager</h1>
        <p>Search the catalog, add servers you need, and disable ones you no longer use.</p>
      </header>

      {(decorated.message || inlineError) && (
        <div className={`mcp-status ${inlineError ? "error" : statusClass}`} role="status">
          {inlineError ?? decorated.message}
        </div>
      )}

      <section className="mcp-search-card" aria-label="Search catalog">
        <form onSubmit={handleSearch}>
          <label htmlFor="mcp-search-input">Search catalog servers</label>
          <div className="mcp-search-row">
            <input
              id="mcp-search-input"
              type="text"
              placeholder="Search by server name, capability, or description"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              autoComplete="off"
              disabled={isSearching}
            />
            <button type="submit" disabled={isSearching}>
              {isSearching ? "Searching…" : "Search"}
            </button>
          </div>
        </form>
        <div>
          <strong>Active servers:</strong>
          {activeServers.length === 0 ? (
            <span className="mcp-empty"> None enabled yet.</span>
          ) : (
            <div className="mcp-active-list" aria-live="polite">
              {activeServers.map((name) => (
                <span key={name} className="mcp-chip">
                  {name}
                </span>
              ))}
            </div>
          )}
        </div>
      </section>

      <section aria-label="Search results">
        {results.length === 0 ? (
          <div className="mcp-empty">
            {decorated.query
              ? `No servers matched “${decorated.query}”. Try a different keyword.`
              : "Start by searching to see available servers you can enable."}
          </div>
        ) : (
          <div className="mcp-card-list">
            {results.map((server) => {
              const isAddPending = pendingAction === `add:${server.name}`;
              const isRemovePending = pendingAction === `remove:${server.name}`;
              return (
                <article key={server.name} className="mcp-card" aria-live="polite">
                  <h2>
                    {server.name}
                    {server.isActive && <span className="mcp-badge">Active</span>}
                  </h2>
                  <p>{server.description ?? "No description provided in the catalog."}</p>
                  <dl>
                    {server.type && (
                      <>
                        <dt>Type</dt>
                        <dd>{server.type}</dd>
                      </>
                    )}
                    {server.remoteUrl && (
                      <>
                        <dt>Endpoint</dt>
                        <dd>{server.remoteUrl}</dd>
                      </>
                    )}
                    {server.image && (
                      <>
                        <dt>Image</dt>
                        <dd>{server.image}</dd>
                      </>
                    )}
                    {typeof server.longLived === "boolean" && (
                      <>
                        <dt>Lifecycle</dt>
                        <dd>{server.longLived ? "Long-lived" : "Ephemeral"}</dd>
                      </>
                    )}
                    {server.requiredSecrets && server.requiredSecrets.length > 0 && (
                      <>
                        <dt>Secrets</dt>
                        <dd>{server.requiredSecrets.join(", ")}</dd>
                      </>
                    )}
                  </dl>
                  {server.tools && server.tools.length > 0 && (
                    <div className="tool-list">
                      <strong>Tools</strong>
                      {server.tools.slice(0, 3).map((tool) => (
                        <span key={tool.name}>
                          {tool.name}
                          {tool.description ? ` — ${tool.description}` : ""}
                        </span>
                      ))}
                      {server.tools.length > 3 && <span>+{server.tools.length - 3} more tools</span>}
                    </div>
                  )}
                  <div className="mcp-card-actions">
                    <button
                      type="button"
                      className="mcp-action-button primary"
                      onClick={() => handleAdd(server.name)}
                      disabled={server.isActive || pendingAction !== null}
                    >
                      {isAddPending ? "Adding…" : server.isActive ? "Already active" : "Add server"}
                    </button>
                    <button
                      type="button"
                      className="mcp-action-button secondary"
                      onClick={() => handleRemove(server.name)}
                      disabled={!server.isActive || pendingAction !== null}
                    >
                      {isRemovePending ? "Removing…" : "Remove server"}
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </section>

      <footer className="mcp-footer" aria-live="polite">
        {decorated.timestamp ? `Last updated ${new Date(decorated.timestamp).toLocaleString()}` : null}
      </footer>
    </div>
  );
};

function mount(): void {
  injectStyles();
  const container = document.getElementById(ROOT_ELEMENT_ID);
  if (!container) {
    console.error("MCP Tool Manager UI failed to mount: root element not found.");
    return;
  }
  if (container.dataset.mounted === "true") {
    return;
  }
  container.dataset.mounted = "true";
  const root = createRoot(container);
  root.render(<App />);
}

if (typeof window !== "undefined") {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => mount(), { once: true });
  } else {
    mount();
  }
}
