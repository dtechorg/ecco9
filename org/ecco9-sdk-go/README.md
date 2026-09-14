# ecco9-sdk-go

Shared Go SDK for the ecco9 distributed cognitive architecture. Every
cognitive service repository (`ecco9-reservoir`, `ecco9-memory`,
`ecco9-echobeats`, ...) depends on this module instead of duplicating
contract and middleware code.

## Contents

```
gen/         gRPC stubs generated from org/ecco9-proto (buf generate)
contracts/   Hand-written Go types mirroring the v1 protobuf messages,
             kept so services compile offline against stable contracts
middleware/  Shared gRPC interceptors: tracing, auth, circuit breaking
health/      Standard /healthz reporter (status, identity coherence, load)
eventmesh/   CloudEvents envelope + echo.* topic taxonomy for NATS JetStream
cognitive/   Cognitive load index computation (the autoscaling signal)
```

## Usage

### Serving

```go
import (
    "github.com/dtechorg/ecco9-sdk-go/health"
    "github.com/dtechorg/ecco9-sdk-go/middleware"
    "google.golang.org/grpc"
)

reporter := health.NewReporter()
server := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        middleware.UnaryServerTracing(),
        middleware.UnaryServerAuth(validator),
    ),
    grpc.ChainStreamInterceptor(
        middleware.StreamServerTracing(),
        middleware.StreamServerAuth(validator),
    ),
)
// Register generated service stubs from gen/, expose reporter.Handler()
// on the telemetry port, and keep reporter.SetCoherence() updated so the
// deployment pipeline's identity-coherence gate can gate rollouts.
```

### Calling other services

```go
breaker := middleware.NewCircuitBreaker()
conn, err := grpc.NewClient(target,
    grpc.WithChainUnaryInterceptor(
        middleware.UnaryClientTracing(),
        middleware.UnaryClientAuth(token),
        middleware.UnaryClientBreaker(breaker),
    ),
    grpc.WithChainStreamInterceptor(
        middleware.StreamClientTracing(),
        middleware.StreamClientAuth(token),
        middleware.StreamClientBreaker(breaker),
    ),
)
```

Only transport-level failures (`Unavailable`, `DeadlineExceeded`,
`Internal`) trip the circuit breaker; caller errors such as
`InvalidArgument` reset it, so a misbehaving client cannot open the circuit
for a healthy callee.

### Publishing cognitive events

```go
import "github.com/dtechorg/ecco9-sdk-go/eventmesh"

env := eventmesh.New("//ecco9/reservoir/instance-0",
    "ecco9.echo.memories.v1", identityID, payload)
err := publisher.Publish(eventmesh.TopicMemories, env)
```

Envelopes follow the CloudEvents 1.0 specification; see
`org/ecco9-eventmesh/schemas/cloudevent.json`.

## Regenerating stubs

The `gen/` tree is the output of `buf generate` run in `org/ecco9-proto`
(see `ecco9-proto/buf.gen.yaml`). Never edit generated files by hand;
change the `.proto` definitions and regenerate.

## Design decisions

1. gRPC for synchronous cognition (inference, state queries); NATS
   JetStream for asynchronous events (memory consolidation, dreams,
   learning).
2. Identity coherence is a first-class metric: every service reports it
   via `health.Reporter`, and rollouts auto-rollback below threshold.
3. Cognitive load (thought queue depth, consolidation backlog, dream
   freshness, coherence delta) — not CPU — is the autoscaling signal.
