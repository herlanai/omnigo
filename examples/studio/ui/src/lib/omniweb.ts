export const protocolVersion = 1 as const;

export type Request = { v: 1; id: string; method: string; params: unknown };
export type Fault = { code: string; message: string };
export type Response = { v: 1; id: string } & (
  | { result: unknown; error?: never }
  | { error: Fault; result?: never }
);

export class OmniWebError extends Error {
  readonly code: string;
  constructor(code: string, message: string) {
    super(message);
    this.name = "OmniWebError";
    this.code = code;
  }
}

// Native WebView adapters can implement the same boundary once available.
export interface Transport {
  exchange(request: Request | Request[], signal: AbortSignal): Promise<unknown>;
}

export class HTTPTransport implements Transport {
  private readonly fetcher: typeof fetch;
  private readonly base: string;
  private readonly maxResponseBytes: number;

  constructor(options: { base?: string; fetch?: typeof fetch; maxResponseBytes?: number } = {}) {
    this.base = options.base ?? "/omniweb/v1";
    if (!this.base.startsWith("/") || this.base.startsWith("//") || /[?#\\]/.test(this.base)) {
      throw new OmniWebError("configuration", "Use a same-origin protocol path");
    }
    this.fetcher = options.fetch ?? globalThis.fetch.bind(globalThis);
    this.maxResponseBytes = options.maxResponseBytes ?? 4 * 1024 * 1024;
    if (!Number.isSafeInteger(this.maxResponseBytes) || this.maxResponseBytes < 1) {
      throw new OmniWebError("configuration", "Invalid response limit");
    }
  }

  async exchange(request: Request | Request[], signal: AbortSignal): Promise<unknown> {
    const body = JSON.stringify(request);
    if (new TextEncoder().encode(body).byteLength > 1024 * 1024) {
      throw new OmniWebError("too_large", "Request exceeds 1 MiB");
    }
    const response = await this.fetcher(`${this.base}/${Array.isArray(request) ? "batch" : "call"}`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      credentials: "same-origin", redirect: "error", body, signal,
    });
    if (!response.ok) {
      await response.body?.cancel();
      throw new OmniWebError(`http_${response.status}`, "OmniWeb transport rejected the request");
    }
    if (!response.headers.get("Content-Type")?.toLowerCase().startsWith("application/json") || !response.body) {
      await response.body?.cancel();
      throw new OmniWebError("protocol", "Expected a JSON response");
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder("utf-8", { fatal: true });
    let bytes = 0;
    let text = "";
    try {
      while (true) {
        const chunk = await reader.read();
        if (chunk.done) break;
        bytes += chunk.value.byteLength;
        if (bytes > this.maxResponseBytes) throw new OmniWebError("too_large", "Response exceeds limit");
        text += decoder.decode(chunk.value, { stream: true });
      }
      text += decoder.decode();
      return JSON.parse(text) as unknown;
    } finally {
      await reader.cancel().catch(() => {});
      reader.releaseLock();
    }
  }
}

function validateResponse(value: unknown, id: string): Response {
  if (!value || typeof value !== "object") throw new OmniWebError("protocol", "Invalid response");
  const r = value as Record<string, unknown>;
  const result = Object.hasOwn(r, "result");
  const error = Object.hasOwn(r, "error");
  if (r.v !== protocolVersion || r.id !== id || result === error) {
    throw new OmniWebError("protocol", "Response version, ID or outcome mismatch");
  }
  if (error) {
    const fault = r.error as Partial<Fault> | null;
    if (!fault || typeof fault.code !== "string" || typeof fault.message !== "string") {
      throw new OmniWebError("protocol", "Invalid fault");
    }
  }
  return value as Response;
}

export type CallOptions = { signal?: AbortSignal; timeoutMs?: number };
type Contract<API> = { [K in keyof API]: { input: unknown; output: unknown } };

export class OmniWebClient<API extends Contract<API>> {
  private readonly transport: Transport;
  private readonly maxPending: number;
  private pending = 0;
  private nextID = 0;

  constructor(transport: Transport = new HTTPTransport(), maxPending = 64) {
    if (!Number.isSafeInteger(maxPending) || maxPending < 1) throw new OmniWebError("configuration", "Invalid pending limit");
    this.transport = transport;
    this.maxPending = maxPending;
  }

  async call<K extends keyof API & string>(method: K, params: API[K]["input"], options: CallOptions = {}): Promise<API[K]["output"]> {
    const timeout = options.timeoutMs ?? 30_000;
    if (!Number.isFinite(timeout) || timeout <= 0 || timeout > 600_000) throw new OmniWebError("configuration", "Invalid timeout");
    if (options.signal?.aborted) throw options.signal.reason;
    if (this.pending >= this.maxPending) throw new OmniWebError("busy", "Client concurrency limit reached");
    if (!/^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$/.test(method) || params === undefined) throw new OmniWebError("invalid_request", "Method and parameters are required");
    const controller = new AbortController();
    const abort = () => controller.abort(options.signal?.reason);
    options.signal?.addEventListener("abort", abort, { once: true });
    const timer = setTimeout(() => controller.abort(new OmniWebError("deadline", "Request timed out")), timeout);
    this.pending++;
    const id = String(++this.nextID);
    try {
      const value = await this.transport.exchange({ v: protocolVersion, id, method, params }, controller.signal);
      const response = validateResponse(value, id);
      if (response.error) throw new OmniWebError(response.error.code, response.error.message);
      // Contract typing is compile-time only. Applications may validate service
      // results with a schema when connecting to an untrusted remote host.
      return response.result as API[K]["output"];
    } finally {
      clearTimeout(timer);
      options.signal?.removeEventListener("abort", abort);
      this.pending--;
    }
  }
}
