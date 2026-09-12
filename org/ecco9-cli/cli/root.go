package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const usage = `ecco9 - Deep Tree Echo CLI (Ollama-compatible shape)

Usage:
  ecco9 serve                       Print the gateway address this CLI targets
  ecco9 generate <prompt>           Generate text via /api/generate
  ecco9 chat                        Interactive chat via /api/chat
  ecco9 tags                        List models via /api/tags
  ecco9 echo status                 Aggregate Deep Tree Echo status
  ecco9 echo think <prompt>         Deep cognitive processing
  ecco9 echo feel <emotion> [n]     Update emotional state (intensity 0-1)
  ecco9 echo remember <key> <value> Store a memory
  ecco9 agents list                 List cognitive agents
  ecco9 orchestrate <input>         Submit an orchestration task
  ecco9 version                     Print gateway and CLI version

Flags:
  -host    Gateway base URL (default ECCO9_HOST or http://127.0.0.1:8080)
  -key     API key (default ECCO9_API_KEY)
  -model   Model name for generate/chat
`

// Execute dispatches argv and returns the process exit code.
func Execute() int {
	fs := flag.NewFlagSet("ecco9", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	host := fs.String("host", "", "gateway base URL")
	key := fs.String("key", "", "API key")
	model := fs.String("model", "", "model name")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	args := fs.Args()
	if len(args) == 0 {
		fmt.Print(usage)
		return 0
	}

	client, err := ClientFromEnvironment()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ecco9:", err)
		return 1
	}
	if *host != "" {
		if client, err = NewClient(*host); err != nil {
			fmt.Fprintln(os.Stderr, "ecco9:", err)
			return 1
		}
	}
	if *key != "" {
		client.APIKey = *key
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch args[0] {
	case "serve":
		fmt.Println("gateway:", client.BaseURL())
		fmt.Println("(start the gateway with ecco9-gateway/cmd/gatewayd)")
		return 0
	case "generate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ecco9 generate <prompt>")
			return 2
		}
		return runGenerate(ctx, client, *model, strings.Join(args[1:], " "))
	case "chat":
		return runChat(ctx, client, *model)
	case "tags":
		return runTags(ctx, client)
	case "echo":
		return runEcho(ctx, client, args[1:])
	case "agents":
		if len(args) < 2 || args[1] != "list" {
			fmt.Fprintln(os.Stderr, "usage: ecco9 agents list")
			return 2
		}
		return runAgentsList(ctx, client)
	case "orchestrate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ecco9 orchestrate <input>")
			return 2
		}
		return runOrchestrate(ctx, client, strings.Join(args[1:], " "))
	case "version":
		v, err := client.Version(ctx)
		if err != nil {
			fmt.Println("ecco9 cli version 0.1.0 (gateway unreachable:", err, ")")
			return 0
		}
		fmt.Println("ecco9 cli version 0.1.0, gateway version", v)
		return 0
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "ecco9:", err)
	return 1
}

func runGenerate(ctx context.Context, c *Client, model, prompt string) int {
	resp, err := c.Generate(ctx, &GenerateRequest{Model: model, Prompt: prompt})
	if err != nil {
		return fail(err)
	}
	fmt.Println(resp.Response)
	return 0
}

func runChat(ctx context.Context, c *Client, model string) int {
	fmt.Println("Deep Tree Echo chat — type a message, empty line to exit.")
	var history []Message
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(">>> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			break
		}
		history = append(history, Message{Role: "user", Content: line})
		resp, err := c.Chat(ctx, &ChatRequest{Model: model, Messages: history})
		if err != nil {
			return fail(err)
		}
		history = append(history, resp.Message)
		fmt.Println(resp.Message.Content)
	}
	return 0
}

func runTags(ctx context.Context, c *Client) int {
	tags, err := c.Tags(ctx)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("%-40s %-20s %s\n", "NAME", "MODIFIED", "SIZE")
	for _, m := range tags.Models {
		fmt.Printf("%-40s %-20s %d\n", m.Name, m.ModifiedAt, m.Size)
	}
	return 0
}

func runEcho(ctx context.Context, c *Client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ecco9 echo status|think|feel|remember")
		return 2
	}
	switch args[0] {
	case "status":
		st, err := c.EchoStatus(ctx)
		if err != nil {
			return fail(err)
		}
		fmt.Printf("status: %s  identity_coherence: %.3f  services: %d/%d up\n",
			st.Status, st.IdentityCoherence, st.ServicesUp, st.ServicesTotal)
		for _, s := range st.Services {
			mark := "up  "
			if !s.Reachable {
				mark = "down"
			}
			fmt.Printf("  %-14s %-4s coherence=%.2f load=%.2f %s\n",
				s.Service, mark, s.IdentityCoherence, s.CognitiveLoad, s.Error)
		}
		return 0
	case "think":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ecco9 echo think <prompt>")
			return 2
		}
		out, err := c.EchoThink(ctx, strings.Join(args[1:], " "))
		if err != nil {
			return fail(err)
		}
		printJSON(out)
		return 0
	case "feel":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ecco9 echo feel <emotion> [intensity]")
			return 2
		}
		intensity := 0.5
		if len(args) > 2 {
			fmt.Sscanf(args[2], "%f", &intensity)
		}
		out, err := c.EchoFeel(ctx, args[1], intensity)
		if err != nil {
			return fail(err)
		}
		printJSON(out)
		return 0
	case "remember":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: ecco9 echo remember <key> <value>")
			return 2
		}
		out, err := c.EchoRemember(ctx, args[1], strings.Join(args[2:], " "))
		if err != nil {
			return fail(err)
		}
		printJSON(out)
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: ecco9 echo status|think|feel|remember")
		return 2
	}
}

func runAgentsList(ctx context.Context, c *Client) int {
	agents, err := c.ListAgents(ctx)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("%-20s %-24s %s\n", "ID", "NAME", "TYPE")
	for _, a := range agents {
		fmt.Printf("%-20s %-24s %s\n", a.ID, a.Name, a.Type)
	}
	return 0
}

func runOrchestrate(ctx context.Context, c *Client, input string) int {
	out, err := c.Orchestrate(ctx, &OrchestrateRequest{Type: "cognitive", Input: input})
	if err != nil {
		return fail(err)
	}
	printJSON(out)
	return 0
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
