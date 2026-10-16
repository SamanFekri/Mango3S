package ai

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveProviders runs real providers when asked to (they use quota):
//
//	MANGO3S_LIVE_CLAUDE_CODE=1 go test ./internal/ai -run Live
//	MANGO3S_LIVE_LOCAL=http://localhost:11434 MANGO3S_LIVE_LOCAL_MODEL=qwen2.5-coder:7b go test ./internal/ai -run Live
func TestLiveProviders(t *testing.T) {
	req := Request{DB: "shop", Coll: "orders", Prompt: "orders with status \"A\" and qty over 5, newest first", Mode: "auto"}
	c := Context{Collection: "shop.orders", Fields: []FieldSummary{
		{Path: "_id", Types: []string{"objectId"}, Presence: 100}, {Path: "status", Types: []string{"string"}, Presence: 100},
		{Path: "qty", Types: []string{"int32"}, Presence: 98}, {Path: "createdAt", Types: []string{"date"}, Presence: 100},
	}}
	run := func(t *testing.T, p Provider) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		start := time.Now()
		raw, u, err := p.Generate(ctx, SystemPrompt, UserMessage(req, c), AnswerSchema())
		if err != nil {
			t.Fatal(err)
		}
		prop, err := Validate(ExtractJSON(raw), req)
		if err != nil {
			t.Fatalf("%v\nraw: %s", err, raw)
		}
		t.Logf("%s in %s: kind=%s valid=%v filter=%s sort=%s (%d/%d tokens)", u.Model, time.Since(start).Round(time.Millisecond), prop.Kind, prop.Valid, prop.Filter, prop.Sort, u.InputTokens, u.OutputTokens)
		if !prop.Valid {
			t.Fatalf("invalid proposal: %+v", prop.Validation)
		}
	}
	t.Run("claude code", func(t *testing.T) {
		if os.Getenv("MANGO3S_LIVE_CLAUDE_CODE") == "" {
			t.Skip("set MANGO3S_LIVE_CLAUDE_CODE=1")
		}
		info := DetectClaudeCode(context.Background(), "")
		if !info.Found {
			t.Skip(info.Message)
		}
		run(t, &ClaudeCode{Path: info.Path})
	})
	t.Run("local", func(t *testing.T) {
		url := os.Getenv("MANGO3S_LIVE_LOCAL")
		if url == "" {
			t.Skip("set MANGO3S_LIVE_LOCAL and MANGO3S_LIVE_LOCAL_MODEL")
		}
		run(t, &Local{URL: url, API: LocalOllama, Model: os.Getenv("MANGO3S_LIVE_LOCAL_MODEL")})
	})
}
