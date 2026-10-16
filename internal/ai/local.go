package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mango3s/internal/apperr"
)

// Local-model APIs.
const (
	LocalOllama = "ollama" // Ollama's native API (/api/chat)
	LocalOpenAI = "openai" // OpenAI-compatible (/v1/chat/completions): LM Studio, llama.cpp, vLLM, Jan, Ollama's /v1
)

// LocalServer is a model server found on this computer.
type LocalServer struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	API    string   `json:"api"`
	Models []string `json:"models"`
}

// HTTPClient is used for local model servers (replaceable in tests).
var HTTPClient = &http.Client{}

// knownServers are the default addresses of popular local model servers.
var knownServers = []LocalServer{
	{Name: "Ollama", URL: "http://localhost:11434", API: LocalOllama},
	{Name: "LM Studio", URL: "http://localhost:1234", API: LocalOpenAI},
	{Name: "llama.cpp server", URL: "http://localhost:8080", API: LocalOpenAI},
	{Name: "vLLM", URL: "http://localhost:8000", API: LocalOpenAI},
	{Name: "Jan", URL: "http://localhost:1337", API: LocalOpenAI},
}

// DetectLocalServers probes the usual local addresses and lists their models.
func DetectLocalServers(ctx context.Context) []LocalServer {
	out := []LocalServer{}
	type res struct {
		i int
		s LocalServer
		e error
	}
	ch := make(chan res, len(knownServers))
	for i, s := range knownServers {
		go func(i int, s LocalServer) {
			cctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			defer cancel()
			models, err := ListLocalModels(cctx, s.URL, s.API, "")
			s.Models = models
			ch <- res{i, s, err}
		}(i, s)
	}
	found := make([]*LocalServer, len(knownServers))
	for range knownServers {
		r := <-ch
		if r.e == nil {
			s := r.s
			found[r.i] = &s
		}
	}
	for _, s := range found {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// IsLoopback reports whether a server URL points at this computer.
func IsLoopback(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := u.Hostname()
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func checkURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, apperr.Invalid("The local model server URL must look like http://localhost:11434.")
	}
	return u, nil
}

func doJSON(ctx context.Context, method, endpoint, apiKey string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rd)
	if err != nil {
		return apperr.Wrap(apperr.CodeInvalidInput, err, "Invalid model server URL.")
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return apperr.From(ctx.Err())
		}
		return apperr.Wrap(apperr.CodeUnreachable, err, "Could not reach the model server. Is it running?")
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error any `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		detail := fmt.Sprint(e.Error)
		if m, ok := e.Error.(map[string]any); ok {
			detail = fmt.Sprint(m["message"])
		}
		if e.Error == nil {
			detail = strings.TrimSpace(string(data))
		}
		if len(detail) > 300 {
			detail = detail[:300]
		}
		switch resp.StatusCode {
		case 401, 403:
			return apperr.New(apperr.CodeAuthFailed, "The model server rejected the request (HTTP %d). Check its API key.", resp.StatusCode)
		case 404:
			return &httpStatus{code: 404, err: apperr.New(apperr.CodeNotFound, "The model server has no such model or endpoint: %s", detail)}
		}
		return &httpStatus{code: resp.StatusCode, err: apperr.New(apperr.CodeMongo, "The model server returned HTTP %d: %s", resp.StatusCode, detail)}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return apperr.New(apperr.CodeMongo, "The model server sent an unexpected response.")
	}
	return nil
}

type httpStatus struct {
	code int
	err  *apperr.Error
}

func (h *httpStatus) Error() string { return h.err.Error() }
func (h *httpStatus) Unwrap() error { return h.err }

// ListLocalModels lists the models a server offers.
func ListLocalModels(ctx context.Context, baseURL, api, apiKey string) ([]string, error) {
	u, err := checkURL(baseURL)
	if err != nil {
		return nil, err
	}
	base := u.String()
	models := []string{}
	if api == LocalOllama {
		var r struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := doJSON(ctx, http.MethodGet, base+"/api/tags", apiKey, nil, &r); err != nil {
			return nil, err
		}
		for _, m := range r.Models {
			models = append(models, m.Name)
		}
		return models, nil
	}
	var r struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := doJSON(ctx, http.MethodGet, openAIBase(base)+"/models", apiKey, nil, &r); err != nil {
		return nil, err
	}
	for _, m := range r.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// openAIBase accepts both http://host:1234 and http://host:1234/v1.
func openAIBase(base string) string {
	if strings.HasSuffix(base, "/v1") {
		return base
	}
	return base + "/v1"
}

// Local generates answers with a model served on this computer (or a server
// the user chose). Structured output is requested with the server's JSON
// schema support; the answer is validated like any other provider's.
type Local struct {
	URL    string
	API    string
	Model  string
	APIKey string // optional, for OpenAI-compatible servers that need one
}

// Generate implements Provider.
func (l *Local) Generate(ctx context.Context, system, user string, schema map[string]any) (string, Usage, error) {
	if strings.TrimSpace(l.Model) == "" {
		return "", Usage{}, apperr.Invalid("Choose a local model in Settings → AI.")
	}
	u, err := checkURL(l.URL)
	if err != nil {
		return "", Usage{}, err
	}
	base := u.String()
	// Not every server enforces the schema, so the model also sees it.
	schemaJSON, _ := json.Marshal(schema)
	system += "\n\nReply with only a JSON object (no prose, no code fence) matching this JSON schema:\n" + string(schemaJSON)
	msgs := []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}
	usage := Usage{Model: l.Model}
	if l.API == LocalOllama {
		var r struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			PromptEvalCount int64 `json:"prompt_eval_count"`
			EvalCount       int64 `json:"eval_count"`
		}
		body := map[string]any{"model": l.Model, "messages": msgs, "stream": false, "format": schema, "options": map[string]any{"temperature": 0}}
		if err := doJSON(ctx, http.MethodPost, base+"/api/chat", l.APIKey, body, &r); err != nil {
			return "", usage, err
		}
		usage.InputTokens, usage.OutputTokens = r.PromptEvalCount, r.EvalCount
		return r.Message.Content, usage, nil
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	body := map[string]any{"model": l.Model, "messages": msgs, "temperature": 0,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "answer", "strict": true, "schema": schema}}}
	err = doJSON(ctx, http.MethodPost, openAIBase(base)+"/chat/completions", l.APIKey, body, &r)
	var hs *httpStatus
	if err != nil && asStatus(err, &hs) && hs.code == 400 {
		// Some servers don't support json_schema; ask for plain JSON instead.
		body["response_format"] = map[string]any{"type": "json_object"}
		err = doJSON(ctx, http.MethodPost, openAIBase(base)+"/chat/completions", l.APIKey, body, &r)
	}
	if err != nil {
		return "", usage, err
	}
	if len(r.Choices) == 0 {
		return "", usage, apperr.New(apperr.CodeMongo, "The model returned no answer.")
	}
	if r.Choices[0].FinishReason == "length" {
		return "", usage, apperr.New(apperr.CodeMongo, "The model's answer was cut off; try a simpler request or a larger context.")
	}
	usage.InputTokens, usage.OutputTokens = r.Usage.PromptTokens, r.Usage.CompletionTokens
	return r.Choices[0].Message.Content, usage, nil
}

func asStatus(err error, target **httpStatus) bool {
	for e := err; e != nil; {
		if h, ok := e.(*httpStatus); ok {
			*target = h
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

// ExtractJSON pulls the JSON object out of a model answer that may be
// wrapped in prose or a ``` fence (common with local models).
func ExtractJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return s
	}
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		if j := strings.Index(rest, "```"); j >= 0 {
			if inner := strings.TrimSpace(rest[:j]); strings.HasPrefix(inner, "{") {
				return inner
			}
		}
	}
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}
