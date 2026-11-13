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

export type EventSubscription = (() => void) & {
  /** Resolves after the server has registered the subscription. */
  readonly ready: Promise<void>;
  /** Resolves on unsubscribe; rejects on transport/protocol/listener failure. */
  readonly done: Promise<void>;
};
export type EventOptions = { signal?: AbortSignal; readyTimeoutMs?: number };

/** Same-origin Go event subscriptions. No implicit reconnect/replay is promised. */
export class OmniWebEvents<Events extends Record<string, unknown>> {
  private readonly base: string;
  private readonly fetcher: typeof fetch;
  private readonly subscriptions = new Set<EventSubscription>();
  private closed = false;

  constructor(options: { base?: string; fetch?: typeof fetch } = {}) {
    this.base = options.base ?? "/omniweb/v1";
    if (!this.base.startsWith("/") || this.base.startsWith("//") || /[?#\\]/.test(this.base)) {
      throw new OmniWebError("configuration", "Use a same-origin protocol path");
    }
    this.fetcher = options.fetch ?? globalThis.fetch.bind(globalThis);
  }
  on<K extends keyof Events & string>(name: K, listener: (value: Events[K]) => void, options: EventOptions = {}): EventSubscription {
    return this.subscribe(name, listener, false, options);
  }
  once<K extends keyof Events & string>(name: K, listener: (value: Events[K]) => void, options: EventOptions = {}): EventSubscription {
    return this.subscribe(name, listener, true, options);
  }
  event<K extends keyof Events & string>(name: K) {
    return { name, on: (listener: (value: Events[K]) => void, options?: EventOptions) => this.on(name, listener, options),
      once: (listener: (value: Events[K]) => void, options?: EventOptions) => this.once(name, listener, options) };
  }
  close(): void {
    this.closed = true;
    for (const off of this.subscriptions) off();
  }
  private subscribe<K extends keyof Events & string>(name: K, listener: (value: Events[K]) => void, once: boolean, options: EventOptions): EventSubscription {
    if (this.closed) throw new OmniWebError("closed", "Event client is closed");
    if (this.subscriptions.size >= 64) throw new OmniWebError("busy", "Event subscription limit reached");
    if (!/^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$/.test(name) || typeof listener !== "function") throw new OmniWebError("invalid_request", "Invalid event subscription");
    const timeout = options.readyTimeoutMs ?? 30_000;
    if (!Number.isFinite(timeout) || timeout <= 0 || timeout > 600_000) throw new OmniWebError("configuration", "Invalid ready timeout");
    const controller = new AbortController();
    let stopped = false;
    let isReady = false;
    let listenerFailed = false;
    let resolveReady!: () => void;
    let rejectReady!: (error: unknown) => void;
    const ready = new Promise<void>((resolve, reject) => { resolveReady = resolve; rejectReady = reject; });
    void ready.catch(() => {});
    const off = (() => { stopped = true; controller.abort(); }) as EventSubscription;
    const externalAbort = () => off();
    options.signal?.addEventListener("abort", externalAbort, { once: true });
    if (options.signal?.aborted) off();
    const timer = setTimeout(() => controller.abort(new OmniWebError("deadline", "Event subscription did not become ready")), timeout);
    const done = (async () => {
      let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
      const cancelRead = () => { void reader?.cancel().catch(() => {}); };
      controller.signal.addEventListener("abort", cancelRead, { once: true });
      try {
        if (stopped) return;
        const response = await this.fetcher(`${this.base}/events`, { method: "POST", credentials: "same-origin", redirect: "error",
          headers: { "Content-Type": "application/json" }, body: JSON.stringify({ v: protocolVersion, topics: [name] }), signal: controller.signal });
        if (controller.signal.aborted) { await response.body?.cancel(); if (!stopped) throw controller.signal.reason; return; }
        if (!response.ok || response.headers.get("Content-Type")?.split(";")[0].trim().toLowerCase() !== "application/x-ndjson" || !response.body) {
          await response.body?.cancel();
          throw new OmniWebError(response.ok ? "protocol" : `http_${response.status}`, "Event stream rejected");
        }
        reader = response.body.getReader();
        const decoder = new TextDecoder("utf-8", { fatal: true });
        let text = "";
        let bytes = 0;
        while (!stopped) {
          const chunk = await reader.read();
          if (controller.signal.aborted && !stopped) throw controller.signal.reason;
          if (stopped) break;
          if (chunk.done) throw new OmniWebError("disconnected", "Event stream ended without unsubscribe");
          let start = 0;
          for (let i = 0; i <= chunk.value.length; i++) {
            if (i < chunk.value.length && chunk.value[i] !== 10) continue;
            const segment = chunk.value.subarray(start, i);
            bytes += segment.byteLength;
            if (bytes > 1024 * 1024) throw new OmniWebError("too_large", "Event frame exceeds limit");
            text += decoder.decode(segment, { stream: true });
            start = i + 1;
            if (i === chunk.value.length) break;
            text += decoder.decode();
            const frame = JSON.parse(text) as Record<string, unknown> | null;
            text = ""; bytes = 0;
            if (!frame || frame.v !== protocolVersion) throw new OmniWebError("protocol", "Invalid event frame");
            if (frame.kind === "ready" && !isReady) { isReady = true; clearTimeout(timer); resolveReady(); }
            else if (frame.kind === "error" && typeof frame.code === "string") throw new OmniWebError(frame.code, "Event stream closed by server");
            else if (frame.kind === "event" && isReady && frame.topic === name && Object.hasOwn(frame, "data")) {
              if (once) off();
              try { listener(frame.data as Events[K]); } catch (error) { listenerFailed = true; throw error; }
              if (stopped) break;
            } else throw new OmniWebError("protocol", "Unexpected event frame");
          }
        }
      } catch (error) {
        // Listener exceptions must remain observable, including once listeners.
        if (listenerFailed || !stopped) { rejectReady(error); throw error; }
      } finally {
        clearTimeout(timer);
        options.signal?.removeEventListener("abort", externalAbort);
        controller.signal.removeEventListener("abort", cancelRead);
        await reader?.cancel().catch(() => {});
        reader?.releaseLock();
        if (!isReady) rejectReady(new OmniWebError("canceled", "Subscription ended before ready"));
        this.subscriptions.delete(off);
      }
    })();
    Object.defineProperties(off, { ready: { value: ready }, done: { value: done } });
    this.subscriptions.add(off);
    void done.catch(() => {});
    // Already-aborted subscriptions can finish synchronously before registration.
    void done.finally(() => this.subscriptions.delete(off)).catch(() => {});
    return off;
  }
}

export type StreamChannel<Value, Result> = AsyncIterable<Value> & {
  readonly ready: Promise<void>;
  readonly result: Promise<Result>;
  close(): void;
};
type StreamContract<API> = { [K in keyof API]: { input: unknown; value: unknown; output: unknown } };

async function* streamFrames(reader: ReadableStreamDefaultReader<Uint8Array>): AsyncGenerator<Record<string, unknown>> {
  const decoder = new TextDecoder("utf-8", {fatal:true});
  let text = "", bytes = 0;
  while (true) {
    const chunk = await reader.read();
    if (chunk.done) { if (bytes) throw new OmniWebError("protocol", "Truncated stream frame"); return; }
    let start = 0;
    for (let i=0;i<=chunk.value.length;i++) {
      if (i<chunk.value.length && chunk.value[i]!==10) continue;
      const segment=chunk.value.subarray(start,i); start=i+1; bytes+=segment.byteLength;
      if (bytes>2*1024*1024) throw new OmniWebError("too_large", "Stream frame exceeds limit");
      text+=decoder.decode(segment,{stream:true});
      if (i===chunk.value.length) break;
      text+=decoder.decode();
      const frame=JSON.parse(text) as Record<string,unknown>|null;
      text="";bytes=0;
      if (!frame || typeof frame!=="object") throw new OmniWebError("protocol","Invalid stream frame");
      yield frame;
    }
  }
}

/** Request-scoped streaming calls. Consume values before awaiting the final result. */
export class OmniWebStreams<API extends StreamContract<API>> {
  private readonly base: string;
  private readonly fetcher: typeof fetch;
  private readonly active = new Set<() => void>();
  private nextID = 0;
  private closed = false;
  constructor(options: {base?:string;fetch?:typeof fetch}={}) {
    this.base=options.base??"/omniweb/v1";
    if (!this.base.startsWith("/") || this.base.startsWith("//") || /[?#\\]/.test(this.base)) throw new OmniWebError("configuration","Use a same-origin protocol path");
    this.fetcher=options.fetch??globalThis.fetch.bind(globalThis);
  }
  close():void {this.closed=true;for(const close of this.active) close();}
  call<K extends keyof API & string>(method:K,params:API[K]["input"],options:EventOptions={}):StreamChannel<API[K]["value"],API[K]["output"]> {
    if(this.closed) throw new OmniWebError("closed","Stream client closed");
    if(this.active.size>=64) throw new OmniWebError("busy","Stream concurrency limit reached");
    if(!/^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$/.test(method)||params===undefined) throw new OmniWebError("invalid_request","Invalid stream request");
    const timeout=options.readyTimeoutMs??30_000;
    if(!Number.isFinite(timeout)||timeout<=0||timeout>600_000) throw new OmniWebError("configuration","Invalid ready timeout");
    const id=String(++this.nextID);
    const body=JSON.stringify({v:protocolVersion,id,method,params});
    if(new TextEncoder().encode(body).byteLength>1024*1024) throw new OmniWebError("too_large","Stream request exceeds limit");
    const controller=new AbortController();
    let reader:ReadableStreamDefaultReader<Uint8Array>|undefined;
    let isReady=false, ended=false, finished=false, taken=false;
    let failure:unknown;
    let wake:(()=>void)|undefined;
    let token="", sequence=0;
    let pending:{value:API[K]["value"];seq:number;release:()=>void}|undefined;
    let readyResolve!:()=>void,readyReject!:(e:unknown)=>void;
    let resultResolve!:(v:API[K]["output"])=>void,resultReject!:(e:unknown)=>void;
    const ready=new Promise<void>((resolve,reject)=>{readyResolve=resolve;readyReject=reject});
    const result=new Promise<API[K]["output"]>((resolve,reject)=>{resultResolve=resolve;resultReject=reject});
    void ready.catch(()=>{});void result.catch(()=>{});
    const notify=()=>{wake?.();wake=undefined};
    const close=()=>{if(!finished)controller.abort(new OmniWebError("canceled","Stream canceled"))};
    const abort=()=>{failure=controller.signal.reason;pending?.release();pending=undefined;notify();void reader?.cancel().catch(()=>{})};
    controller.signal.addEventListener("abort",abort,{once:true});
    options.signal?.addEventListener("abort",close,{once:true});
    this.active.add(close);
    if(options.signal?.aborted)close();
    const acknowledge=async(seq:number)=>{
      try {
        const response=await this.fetcher(`${this.base}/stream/ack`,{method:"POST",headers:{"Content-Type":"application/json"},credentials:"same-origin",redirect:"error",body:JSON.stringify({v:protocolVersion,token,seq}),signal:controller.signal});
        await response.body?.cancel();
        controller.signal.throwIfAborted();
        // A finished producer retires its session before the browser has necessarily
        // consumed its bounded tail. The ordered result frame still proves success.
        if(response.status!==204&&response.status!==410)throw new OmniWebError("stream_ack","Consumption acknowledgment rejected");
      } catch(error) {controller.abort(error);throw error;}
    };
    const timer=setTimeout(()=>controller.abort(new OmniWebError("deadline","Stream did not become ready")),timeout);
    void (async()=>{
      try{
        controller.signal.throwIfAborted();
        const response=await this.fetcher(`${this.base}/stream`,{method:"POST",headers:{"Content-Type":"application/json"},credentials:"same-origin",redirect:"error",body,signal:controller.signal});
        if(controller.signal.aborted){await response.body?.cancel();throw controller.signal.reason}
        if(!response.ok||!response.body||response.headers.get("Content-Type")?.split(";")[0].trim().toLowerCase()!=="application/x-ndjson"){
          await response.body?.cancel();throw new OmniWebError(response.ok?"protocol":`http_${response.status}`,"Stream rejected");
        }
        reader=response.body.getReader();
        for await(const frame of streamFrames(reader)){
          controller.signal.throwIfAborted();
          if(frame.v!==protocolVersion||frame.id!==id)throw new OmniWebError("protocol","Stream correlation mismatch");
          if(frame.kind==="ready"&&!isReady){
            if(typeof frame.token!=="string"||! /^[a-f0-9]{64}$/.test(frame.token))throw new OmniWebError("protocol","Invalid stream token");
            token=frame.token;isReady=true;clearTimeout(timer);readyResolve();}
          else if(frame.kind==="value"&&isReady&&!ended&&Object.hasOwn(frame,"data")){
            if(!Number.isSafeInteger(frame.seq)||frame.seq!==sequence+1)throw new OmniWebError("protocol","Invalid stream sequence");
            sequence++;
            await new Promise<void>(release=>{pending={value:frame.data as API[K]["value"],seq:sequence,release};notify()});
          } else if(frame.kind==="end"&&isReady&&!ended){ended=true;notify();}
          else if(frame.kind==="result"&&ended){
            if(Object.hasOwn(frame,"result")===Object.hasOwn(frame,"error"))throw new OmniWebError("protocol","Invalid stream result");
            if(Object.hasOwn(frame,"error")){const fault=frame.error as Partial<Fault>|null;if(!fault||typeof fault.code!=="string"||typeof fault.message!=="string")throw new OmniWebError("protocol","Invalid stream fault");throw new OmniWebError(fault.code,fault.message)}
            resultResolve(frame.result as API[K]["output"]);finished=true;return;
          } else throw new OmniWebError("protocol","Unexpected stream frame");
        }
        controller.signal.throwIfAborted();
        throw new OmniWebError("disconnected","Stream ended before result");
      }catch(error){failure=controller.signal.aborted?controller.signal.reason:error;readyReject(failure);resultReject(failure)}
      finally{finished=true;pending?.release();pending=undefined;notify();clearTimeout(timer);options.signal?.removeEventListener("abort",close);controller.signal.removeEventListener("abort",abort);await reader?.cancel().catch(()=>{});reader?.releaseLock();this.active.delete(close);}
    })();
    return {ready,result,close,[Symbol.asyncIterator]:()=>{
      if(taken)throw new OmniWebError("busy","Stream already has a consumer");taken=true;
      return (async function*(){
        try{while(true){
          if(pending){const item=pending;pending=undefined;try{await acknowledge(item.seq);}finally{item.release();}yield item.value;continue;}
          if(ended)return;
          if(failure!==undefined)throw failure;
          await new Promise<void>(resolve=>{wake=resolve});
        }}finally{if(!ended)close()}
      })();
    }};
  }
}
