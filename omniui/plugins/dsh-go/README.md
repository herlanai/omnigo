# DeepSeek Harness for OmniGo

`dsh-go` embeds **omnix's agent runtime** in a Go application. It is a harness,
not another Chat client: `agent.Agent` drives model/tool steps, projects model
history from `session.SessionLog`, and executes registered tools through
`tools.Pipeline`. The default model provider is omnix's `provider_deepseek`.
No model/tool loop or DeepSeek SSE parser is implemented by this integration.

## Native chat panel

Create one `Panel` per window. It owns the harness, message list, draft,
streaming updates, tool status, approvals, local feedback, edit-to-draft and
clear action. It uses Go `AIMessage`, `MessageScroller` and `PromptInput`;
there is no WebView or frontend server.

```go
panel, err := dshgo.NewPanel(dshgo.PanelOptions{
    Agent: dshgo.Options{
        DeepSeek: dshgo.DeepSeekOptions{APIKey: os.Getenv("DEEPSEEK_API_KEY")},
        Tools: appTools,
    },
    RunTask: env.RunTask,
    Title: "Assistant",
})
if err != nil { return omnigo.ContentWindowOptions{}, err }
return omnigo.ContentWindowOptions{
    Title: "Assistant", Width: 900, Height: 780,
    TitleBarHidden: func() bool { return true },
    DragRegion: "assistant/title",
    View: func() *ui.Element { return panel.View("assistant") },
    Close: func() { _ = panel.Close() },
}, nil
```

Use this inside `Options.GoWindow`; `env` is its `WindowEnvironment` and
`appTools` is your explicit `[]dshgo.Tool`. The complete runnable example is
[`examples/assistant`](../../../examples/assistant/main.go):

```sh
DEEPSEEK_API_KEY=... go run ./examples/assistant
```

Studio's **dsh-go** plugin demo uses a labeled offline model with real omnix
agent/tool execution. Send a question to exercise Allow/Deny; send `wait` to
exercise Stop. The standalone example uses the real DeepSeek provider.

- Supply `env.RunTask` to dispatch coalesced streaming updates onto the UI.
  Without it the panel renders as a disabled preview. No polling is used.
- `View`, `Send`, `SetDraft`, `Busy`, `Stop`, `Reset`, `Configure`, `Close`,
  `SetNotice`, `SetHeaderActions`, `SetDisabled` and `SetReadOnly` run on the UI
  goroutine. `View` IDs must be stable and unique. Construct once, not per frame.
- Missing `Agent.Approve` installs the panel's per-call approval UI. An explicit
  callback takes precedence. Closing or stopping cancels a pending decision.
- Enter sends; Shift+Enter adds a newline; IME composition does not submit.
  The editor is disabled while running. Read-only/disabled mode stops work and
  prevents new turns. Clear and Configure reject active turns.
- Message scrolling follows the end unless the user has scrolled up; Jump to
  latest restores it. `MaxMessages` defaults to 40 rendered messages and is
  independent of the harness history budget. This is bounded rendering, not a
  virtual list. Give the panel approximately 900 × 780 DIP to accommodate the
  inline approval and composer; very small bounds need application layout work.
- Configure validates before replacing the idle harness and clearing messages.
  SetNotice and SetHeaderActions customize the shell. Provide `Copy` for native
  clipboard integration. Feedback is local view state, not a provider rating API.

### Reference panel mapping and remaining differences

The reference panel's view, busy state, notice, header actions, configuration,
stop, clear, user editing, assistant feedback and one-call approval are mapped
above. Its window attachment is replaced by `RunTask` plus the window `Close`
callback. The same configured-model display is retained; this is not a
multi-provider configuration screen. Message content is plain text as provided
by baseui; rich Markdown, virtual-list performance, native IME/assistive
technology and device-level parity remain unverified or incomplete. No claim
of full reference parity follows from the offscreen tests.

## Minimal agent

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    dshgo "github.com/herlanai/omnigo/omniui/plugins/dsh-go"
)

func main() {
    assistant, err := dshgo.New(dshgo.Options{
        DeepSeek: dshgo.DeepSeekOptions{APIKey: os.Getenv("DEEPSEEK_API_KEY")},
        System: "Use the registered tools when needed.",
        Tools: []dshgo.Tool{{
            Name: "app_version",
            Description: "Read this application's version",
            Execute: func(ctx context.Context, input map[string]any) (any, error) {
                return "1.0.0", nil
            },
        }},
    })
    if err != nil { log.Fatal(err) }
    defer assistant.Close()

    result, err := assistant.Run(context.Background(), "What version is this app?", func(e dshgo.Event) {
        if e.Kind == "text" { fmt.Print(e.Text) }
    })
    if err != nil { log.Fatal(err) }
    fmt.Printf("\n%d model steps, %d output tokens\n", result.Steps, result.Usage.CompletionTokens)
}
```

Set `Options.Adapter` to any omnix `llm.LLMAdapter` to select another provider
or a deterministic test model. The default model is `deepseek-chat`.

## Tools and approvals

Tools are explicit Go functions. Set `Parameters` to a JSON Schema object;
omnix compiles and validates it before execution. Duplicate tool names fail
construction. No shell, filesystem, desktop automation or other privileged
tools are enabled implicitly.

Set `RequiresApproval: true` and provide
`Options.Approve func(context.Context, llm.ToolCall) (bool, error)` for protected
actions. Missing approval denies execution. Denials, invalid arguments, unknown
tools and tool execution errors become tool results for the agent's next step.
A UI approval callback must dispatch its prompt to the UI and wait on a channel
or `ctx.Done()` on the worker.

## Lifetime, history and limits

- `New` starts no agent worker or network request. `Run` is blocking; call it
  from a worker, such as an application task. omnix starts its own worker for
  the turn and it is disposed before `Run` returns.
- Events (`text`, `reasoning`, `tool_start`, `tool_end`) and approval callbacks
  run on the omnix worker. Dispatch all UI state changes to the UI goroutine.
- `Cancel` interrupts the current turn. `Close` prevents future turns and
  cancels current work without blocking the UI. All providers, tools and
  callbacks must cooperate with context cancellation.
- Concurrent `Run` / `Reset` returns `ErrBusy`. `History` waits for the active
  turn and returns an independent omnix session-event snapshot; do not call
  it from a turn callback. `Reset` clears history when idle.
- Successful turns are retained for subsequent requests. Failed turns are not
  committed; their partial events are returned in `Result.Events`. Tools may
  already have performed effects before a later failure.
- Defaults: 8 model steps, 16 tool calls, 2,048 output tokens per request,
  256 KiB turn budget, 4 MiB history/request budget, and a two-minute deadline.
  Byte budgets include serialized metadata. Exceeding a budget returns
  `ErrLimit`; use `Reset` to clear a full history. No silent history truncation
  or automatic compaction is performed.
- `Result` reports text, accumulated usage, model steps and omnix session
  events. Persistence is application-owned; credentials are not put in events.

## Scope and migration

This integration uses omnix agent execution, schemas, tool pipelines, prompt
assembly, credentials and session history. It does not yet expose a session resume API, built-in filesystem/shell tools, skills, subagents,
or omnix's full plan/goal/persistence surface. It does not claim full reference
UI or harness feature parity.

`New(Options)` now creates the harness. The earlier low-level HTTP client remains
available as `NewClient(ClientOptions)` for existing raw-stream integrations;
it is not used to implement agent execution. `NewDeepSeek(DeepSeekOptions)` now
returns omnix's provider directly rather than OmniGo's former custom adapter.

Only importing this optional plugin links omnix; importing the OmniGo root
package does not. Dependency: omnix v0.1.2, [MIT license](licenses/omnix-LICENSE).
