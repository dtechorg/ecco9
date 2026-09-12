# ecco9-cli

Command-line interface and client SDK for the ecco9 platform, keeping the
Ollama CLI shape while adding Deep Tree Echo and orchestration commands. All
traffic goes through the ecco9-gateway REST endpoints.

## Build

```sh
go build ./...
```

Stdlib-only; depends on the local `github.com/dtechorg/ecco9-sdk-go` via a
`replace` directive, so builds work fully offline.

## Usage

```
ecco9 serve                        Print the gateway address this CLI targets
ecco9 generate <prompt>            Generate text via /api/generate
ecco9 chat                         Interactive chat via /api/chat
ecco9 tags                         List models via /api/tags
ecco9 echo status                  Aggregate Deep Tree Echo status
ecco9 echo think <prompt>          Deep cognitive processing
ecco9 echo feel <emotion> [n]      Update emotional state (intensity 0-1)
ecco9 echo remember <key> <value>  Store a memory
ecco9 agents list                  List cognitive agents
ecco9 orchestrate <input>          Submit an orchestration task
ecco9 version                      Print gateway and CLI version
```

Flags: `-host` (default `ECCO9_HOST` or `http://127.0.0.1:8080`), `-key`
(default `ECCO9_API_KEY`), `-model`.

## Client SDK

`cli.Client` exposes typed methods matching the gateway surface:

```go
c, _ := cli.ClientFromEnvironment()
resp, err := c.Generate(ctx, &cli.GenerateRequest{Model: "echo", Prompt: "hello"})
```

Methods: `Generate`, `Chat`, `Tags`, `Version`, `EchoStatus`, `EchoThink`,
`EchoFeel`, `EchoRemember`, `ListAgents`, `Orchestrate`.
