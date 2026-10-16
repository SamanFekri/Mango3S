package ai

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"mango3s/internal/apperr"
)

// DefaultModel is used unless the user picks another in Settings.
const DefaultModel = "claude-opus-5-5"

// Claude calls the Anthropic Messages API with structured outputs.
type Claude struct {
	client anthropic.Client
	model  string
}

// NewClaude creates a provider. An empty apiKey falls back to the SDK's
// default credential chain (ANTHROPIC_API_KEY, `ant auth login` profiles).
func NewClaude(apiKey, model string) *Claude {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	return &Claude{client: anthropic.NewClient(opts...), model: model}
}

// Generate implements Provider.
func (c *Claude) Generate(ctx context.Context, system, user string, schema map[string]any) (string, Usage, error) {
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 16000,
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(user)),
		},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortMedium,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: schema},
		},
	}
	if supportsDefaultFallback(c.model) {
		// A safety-classifier decline is re-served by a fallback model inside the same call.
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: "default"}
	}
	msg, err := c.client.Beta.Messages.New(ctx, params)
	if err != nil {
		return "", Usage{}, classify(err)
	}
	u := Usage{InputTokens: msg.Usage.InputTokens, OutputTokens: msg.Usage.OutputTokens, Model: string(msg.Model)}
	switch msg.StopReason {
	case anthropic.BetaStopReasonRefusal:
		return "", u, apperr.New(apperr.CodeUnsupported, "The model declined this request.")
	case anthropic.BetaStopReasonMaxTokens:
		return "", u, apperr.New(apperr.CodeMongo, "The AI response was cut off; try a simpler request.")
	}
	var out strings.Builder
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			out.WriteString(t.Text)
		}
	}
	return out.String(), u, nil
}

// supportsDefaultFallback reports whether fallbacks: "default" applies.
func supportsDefaultFallback(model string) bool {
	switch model {
	case "claude-opus-5-5", "claude-opus-5", "claude-fable-5-1", "claude-sonnet-5-5":
		return true
	}
	return false
}

func classify(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401, 403:
			return apperr.New(apperr.CodeAuthFailed, "The Anthropic API key was rejected. Update it in Settings → AI.")
		case 404:
			return apperr.New(apperr.CodeInvalidInput, "The configured model is not available to this API key.")
		case 429:
			return apperr.New(apperr.CodeTimeout, "The AI provider is rate limiting requests; try again shortly.")
		}
		if apiErr.StatusCode >= 500 {
			return apperr.New(apperr.CodeUnreachable, "The AI provider is temporarily unavailable.")
		}
		return apperr.New(apperr.CodeMongo, "The AI request failed (HTTP %d).", apiErr.StatusCode)
	}
	return apperr.Wrap(apperr.CodeUnreachable, err, "Could not reach the AI provider.")
}
