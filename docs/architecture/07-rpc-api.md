## 7. RPC API (Connect, initial)

Defined in `proto/celine/v1/celine.proto` — one source of truth, codegen for both sides via `buf generate`.

```proto
service CelineService {
  // Send a message; server streams the reply (tokens + tool activity).
  rpc Chat(ChatRequest) returns (stream ChatEvent);

  // Load a conversation's messages.
  rpc GetHistory(GetHistoryRequest) returns (GetHistoryResponse);

  // List conversations.
  rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);

  // Resolve the authenticated caller (verify bearer token + upsert client).
  rpc GetCurrentUser(GetCurrentUserRequest) returns (User);
}

message ChatRequest {
  string conversation_id = 1;  // empty = start a new conversation
  string text = 2;
}

message GetCurrentUserRequest {}

message User {
  string sub = 1;            // Google `sub` — the stable per-client identity
  string email = 2;
  string display_name = 3;
}

// One streamed event. `oneof` keeps the stream typed end to end.
// Chat persona delivers whole bubbles with typing beats, not token deltas (see §14).
message ChatEvent {
  oneof event {
    Typing      typing       = 1;  // "…" indicator before a bubble
    Message     message      = 2;  // one complete chat bubble
    ToolCall    tool_call    = 3;  // tool started
    ToolResult  tool_result  = 4;  // tool finished
    Done        done         = 5;  // final, stream closes after
    string      error        = 6;
  }
}

message Typing  { int32 ms_hint = 1; }            // how long to show the indicator
message Message { int32 seq = 1; string text = 2; } // a whole bubble, in order
```

- **`Chat`** is a **server-streaming RPC**: the browser sends one request, the backend streams `ChatEvent`s until done. This replaces SSE entirely — the stream is typed, generated, and identical on both ends.
- The React side consumes it with the generated client (`for await (const event of client.chat(req))`), switching on the `oneof`.
- Connect speaks plain HTTP/JSON in the browser (no gRPC infra needed), and the same service is callable via gRPC/grpc-web later for free.

### 7.1 Auth — client-side redirect, bearer on every call

The OAuth redirect happens **client-side**: the React app runs the Google sign-in flow (Google Identity Services) and receives a Google ID token. The Go backend exposes **no `/login` and no `/callback`** redirect endpoints — it only verifies.

- The client attaches `Authorization: Bearer <google_id_token>` to **every** RPC (a Connect client interceptor).
- A server interceptor (`basis/internal/auth/google.go`) verifies the token against Google's JWKS, extracts the `sub` claim, and puts the caller identity on the context. Unauthenticated calls fail with `CodeUnauthenticated`.
- Because identity comes from the verified token, **no request message carries an owner field** — `ChatRequest`, `ListConversationsRequest`, etc. stay identity-free; the handler reads `sub` from context.
- `GetCurrentUser` is the single auth-flow RPC: the SPA calls it once after the redirect to verify the session, upsert the `clients` row, and load the profile. Sign-out is client-side (drop the token) — there is no server session store (§12).
- *(Not wired yet — arrives with the auth milestone; until then handlers run unauthenticated in dev.)*


