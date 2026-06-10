## 7. RPC API (Connect)

Defined in `proto/celine/v1/*.proto` — one source of truth, codegen for both
sides via `buf generate`. Two services: **`Celine`** (the chat surface) and
**`Hermes`** (the auth flow, §7.1). Method names are Greek (see naming
convention); the `RPC_RESPONSE_STANDARD_NAME` lint is excepted in `buf.yaml`
because `Laleo` streams a typed `LaleoEvent` oneof, not a `LaleoResponse`.

```proto
service Celine {
  // Send a message; server streams the reply (bubbles + tool activity).
  rpc Laleo     (LaleoRequest)     returns (stream LaleoEvent);
  // Load the caller's conversation history (oldest first).
  rpc Anamnesis (AnamnesisRequest) returns (AnamnesisResponse);
}

message LaleoRequest { string text = 1; }   // identity-free — sub comes from the token

// One streamed event. `oneof` keeps the stream typed end to end.
// Celine delivers whole bubbles, not token deltas (see §14); there is no
// Typing event — the backend sends each complete bubble as it is ready.
message LaleoEvent {
  oneof event {
    Message    message     = 2;  // one complete chat bubble
    ToolCall   tool_call   = 3;  // tool started
    ToolResult tool_result = 4;  // tool finished
    Done       done        = 5;  // final; carries conversation_id, stream closes after
    string     error       = 6;
  }
}

message Message    { int32 seq = 1; string text = 2; }   // a whole bubble, in order
message ToolCall   { string id = 1; string name = 2; string input_json = 3; }
message ToolResult { string id = 1; string output = 2; bool is_error = 3; }
message Done       { int64 conversation_id = 1; }

message AnamnesisRequest {}
message AnamnesisResponse { repeated ChatMessage messages = 1; }
message ChatMessage {
  int64 id = 1; int64 prosopon_id = 2; string text = 3;
  google.protobuf.Timestamp created_at = 4;
}
```

- **`Laleo`** is a **server-streaming RPC**: the browser sends one request, the
  backend streams `LaleoEvent`s until `Done`. This replaces SSE entirely — the
  stream is typed, generated, and identical on both ends.
- The React side consumes it with the generated client
  (`for await (const ev of celine.laleo(req))`), switching on the `oneof`.
- **No request carries a conversation or owner field.** Identity (`sub`) comes
  from the verified token on the context (§7.1); the active conversation is
  resolved server-side — `Laleo` does `conversations.GetOrCreate(prosopon)`,
  `Anamnesis` reads the conversation ID from the token claim. One conversation
  per client at this stage.
- Connect speaks plain HTTP/JSON in the browser (no gRPC infra needed), and the
  same service is callable via gRPC/grpc-web later for free.

### 7.1 Auth — server-side code exchange, server-issued JWT, bearer on every call

Auth lives in its own **`Hermes`** service. The Go backend **does** run the
OAuth code exchange server-side (it holds the Google client secret) and then
issues its **own** HS256 JWT — the browser never handles a raw Google ID token
after the redirect, and carries the Celine JWT instead.

```proto
service Hermes {
  rpc Eisodos  (EisodosRequest)  returns (EisodosResponse);   // get the Google sign-in URL + state
  rpc Metabole (MetaboleRequest) returns (MetaboleResponse);  // exchange code → Celine JWT + User
  rpc Gnorizo  (GnorizoRequest)  returns (GnorizoResponse);   // resolve the current caller
  rpc Exodos   (ExodosRequest)   returns (ExodosResponse);    // logout (client drops the token)
}

message EisodosResponse  { string url = 1; string state = 2; }
message MetaboleRequest  { string code = 1; string redirect_uri = 2; }
message MetaboleResponse { string token = 1; User user = 2; }
message GnorizoResponse  { User user = 1; }
message User { string sub = 1; string email = 2; string display_name = 3; string avatar_url = 4; }
```

- The SPA calls **`Eisodos`** for the Google sign-in URL, redirects, gets a
  `code` back, then calls **`Metabole{code, redirect_uri}`** — the server swaps
  the code with Google, upserts the `prosopons` row, and returns a Celine JWT
  whose claims embed `sub`, `prosopon_id`, and `conversation_id`.
- The client attaches `Authorization: Bearer <celine_jwt>` to **every** RPC.
  A server interceptor (`basis/internal/hermes/interceptor.go`) verifies it and
  puts `sub` / `prosopon_id` / `conversation_id` on the context. RPCs under
  `/celine.v1.Hermes/` skip verification (the auth flow itself needs no token).
- Because identity comes from the token, **no request message carries an owner
  field** — `LaleoRequest` stays identity-free; handlers read claims from context.
- **`Gnorizo`** resolves the current caller (verify token → load profile);
  sign-out (**`Exodos`**) is client-side (drop the token) — there is no server
  session store (§12).
- **Dev mode:** if `CELINE_JWT_SECRET` is unset the interceptor skips
  verification and treats every caller as `"anon"`, so the stack runs locally
  without Google OAuth.
