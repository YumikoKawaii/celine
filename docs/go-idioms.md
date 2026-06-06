# Go Idioms — a working notes doc

> Scratch space for thinking through Go patterns as we build Celine.
> Not exhaustive — focused on the things that come up in this codebase.

---

## 1. Interfaces belong to the consumer

This is the biggest mental shift coming from Java or TypeScript.

**In Java/C#:** you declare an interface in the producer package, then the implementation `implements` it explicitly. Consumers depend on that interface package.

**In Go:** there is no `implements` keyword. A type satisfies an interface automatically if it has the right methods. The compiler checks at the call site. This means you can define an interface *anywhere* — and the idiomatic place is *where you need it*.

### What that looks like in Celine

`rpc/hermes_service.go` needs to call two methods on a client store:

```go
// defined right here, in the consumer package
type clientStore interface {
    Upsert(ctx context.Context, c mneme.Client) error
    Get(ctx context.Context, sub string) (mneme.Client, error)
}
```

`mneme.ClientRepo` has both of those methods. Go sees that at the call site and it just works — no annotation, no registration, no interface package to import.

`mneme` never declares `ClientRepository`. It just ships `*ClientRepo`.

### Why this matters

1. **The producer doesn't have to predict every consumer.** If tomorrow a new service needs `ClientRepo` but only calls `Get`, it defines its own 1-method interface. The producer hasn't changed.

2. **No circular imports.** A shared `repository` package creates a dependency that every consumer must import. Consumer-defined interfaces have no such coupling.

3. **Smaller interfaces for free.** When you define the interface at the call site, you only put in what you actually call. You can't accidentally make it fat.

---

## 2. The proverb: "accept interfaces, return concrete types"

This is the single most quoted Go design rule. It has two halves:

### Accept interfaces

When a function or constructor takes a dependency, use an interface — even a tiny one — so the caller can substitute a mock, a different implementation, or a future version.

```go
// agent.go
type brain interface {
    StreamChat(ctx context.Context, ...) (llm.Turn, error)
}

func New(b brain, ...) *Agent { ... }
```

`Agent` doesn't import `llm`. It doesn't know `*llm.Client` exists. It only knows "I need something with a `StreamChat` method." In tests you can pass a stub. In production you pass `*llm.Client`. Same code, no friction.

### Return concrete types

When you produce something, return the real type — not an interface:

```go
// mneme/conversations.go
func (r *ConversationRepo) GetOrCreate(...) (string, error) { ... }
```

`*ConversationRepo` is returned by `store.go` as a concrete `*ConversationRepo`, not as `ConversationRepository`. The consumer decides what interface to hold it as. Returning an interface is usually wrong because it hides information the caller might need and makes nil-checking confusing.

**The exception:** when the concrete type is an implementation detail that should never leak. A constructor can return an interface when the type itself is unexported:

```go
// store.go
func New(db *gorm.DB, rdb *redis.Client) UnitOfWork { // returns interface
    return &unitOfWork{...}                            // *unitOfWork is unexported
}
```

This is exactly what `mneme.New` does. `*unitOfWork` is unexported — callers have no business reaching into it. So the constructor returns the `UnitOfWork` interface. This is the one valid use of "return interface."

---

## 3. Small interfaces

The standard library sets the tone:

```go
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type Closer interface { Close() error }
```

One method each. Then composition:

```go
type ReadWriter interface { Reader; Writer }
type ReadWriteCloser interface { Reader; Writer; Closer }
```

The power: `os.File`, `bytes.Buffer`, `net.Conn`, `strings.Reader` — all of them satisfy `io.Reader`. None of them know about each other. An HTTP handler, a CSV parser, a gzip compressor — they all take `io.Reader` and work on any of those types.

### In Celine

`convStore` in `agent.go` has one method. `EventSink` has four. `brain` has one. The agent doesn't know about `*llm.Client`, `*ConversationRepo`, or `streamSink`. It only knows the shapes it needs.

If a type has 8+ methods in an interface, that's a smell — it's probably a Java-style interface that was designed from the producer side.

---

## 4. Implicit satisfaction and what it enables

Because interfaces are satisfied implicitly, you can make a third-party type satisfy your interface with zero changes to that library. You can also make your own type satisfy multiple interfaces from different packages simultaneously — none of those packages know about each other.

### The test case

The main practical payoff is testing. Because `agent.Agent` takes `brain` (interface), a test can do:

```go
type fakeBrain struct { reply string }
func (f *fakeBrain) StreamChat(_ context.Context, _ string, _ []llm.Message, _ []llm.ToolDef, deltas chan<- string) (llm.Turn, error) {
    deltas <- f.reply
    close(deltas)
    return llm.Turn{Text: f.reply}, nil
}
```

No mocking library needed. No `gomock` code generation. Just a struct that has the right method. This is why `agent_test.go` can test the full bubble-pacing logic without an Anthropic API key.

---

## 5. Where NOT to follow the rule

### Package-level constructors sometimes return interfaces

Already covered above: when the concrete type is unexported, the constructor returns the interface. `mneme.New` → `UnitOfWork`.

### Errors are always interfaces

`error` is a built-in interface (`Error() string`). You return `error`, not `*MyError`. Callers use `errors.Is` / `errors.As` to unwrap.

### When you own both sides

If a function is in the same package as its only caller and will never be tested in isolation, a concrete type is fine. Interfaces add indirection — don't add indirection without a reason.

---

## 6. Dependency injection through constructors (not globals)

Go doesn't have a DI framework (Spring, NestJS) and doesn't need one. The pattern is:

1. All dependencies come in through `New(...)`.
2. The struct stores them as unexported fields.
3. `main()` wires everything together in one place.

```go
// agent/agent.go
type Agent struct {
    brain  brain      // unexported — callers can't reach in
    convs  convStore
    msgs   msgStore
    tools  toolRunner
    system string
}

func New(b brain, systemPrompt string, convs convStore, msgs msgStore, tools toolRunner) *Agent {
    return &Agent{brain: b, system: systemPrompt, convs: convs, msgs: msgs, tools: tools}
}
```

```go
// cmd/celine/main.go — the wiring happens here, once, visibly
agent.New(brain, agent.SystemPrompt(), uow.Store().Conversations, uow.Store().Messages, tools)
```

No `init()` side-effects. No package-level `var db *sql.DB`. No service locator. The dependency graph is just function calls in `main`.

---

## 7. Things to watch out for

| Pattern | Problem |
|---|---|
| Interface defined in the producer package | Couples consumers, prevents the producer from returning concrete types |
| Interface with 5+ methods | Usually a sign it was designed top-down; split it |
| `var _ SomeInterface = (*MyType)(nil)` | Compile-time interface check — useful when the satisfaction isn't obvious, but don't litter code with it |
| Returning `interface{}` / `any` | Throws away type info; use generics or a concrete type |
| Nil interface vs nil concrete pointer | A nil `*ClientRepo` assigned to `clientStore` is a **non-nil interface**. Never return a typed nil as an interface — return `nil` untyped. |

The last one is a classic Go trap worth its own example:

```go
// BAD
func getStore() clientStore {
    var r *ClientRepo = nil
    return r // non-nil interface wrapping a nil pointer — panics on method call
}

// GOOD
func getStore() clientStore {
    return nil // truly nil interface
}
```
