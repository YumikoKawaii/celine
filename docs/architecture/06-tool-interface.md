## 6. The Tool interface (so growth is trivial)

```go
type Tool interface {
    Name() string
    Description() string
    Schema() map[string]any        // JSON schema for inputs
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}
```

Add a tool → implement these methods → `registry.Register(...)`. Claude sees it automatically.


