package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	AppID      string
	AppSecret  string
	HTTPClient *http.Client
}

type Identity struct {
	ExternalUserID   string
	ExternalUserName string
}

type ChatRequest struct {
	Identity    Identity
	Key         string
	Content     string
	MaxRows     int
	AutoExecute *bool
}

type chatPayload struct {
	AppID            string `json:"app_id,omitempty"`
	AppSecret        string `json:"app_secret,omitempty"`
	ExternalUserID   string `json:"external_user_id,omitempty"`
	ExternalUserName string `json:"external_user_name,omitempty"`
	Key              string `json:"key,omitempty"`
	Content          string `json:"content"`
	MaxRows          int    `json:"max_rows,omitempty"`
	AutoExecute      *bool  `json:"auto_execute,omitempty"`
}

type Envelope[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	RequestID string `json:"request_id,omitempty"`
}

type ServerChatData struct {
	Session ServerSession `json:"session"`
	Result  ChatResult    `json:"result"`
}

type ServerSession struct {
	AppID            string `json:"app_id"`
	SessionID        uint64 `json:"session_id"`
	SessionKey       string `json:"session_key"`
	ExternalUserID   string `json:"external_user_id"`
	ExternalUserName string `json:"external_user_name,omitempty"`
}

type ChatResult struct {
	Agent      AgentResult       `json:"agent"`
	Execution  *ExecutionResult  `json:"execution,omitempty"`
	Executions []ExecutionResult `json:"executions,omitempty"`
	Loops      int               `json:"loops,omitempty"`
	MaxLoops   int               `json:"max_loops,omitempty"`
}

type AgentResult struct {
	Answer            string       `json:"answer,omitempty"`
	SQL               string       `json:"sql,omitempty"`
	Explanation       string       `json:"explanation,omitempty"`
	NeedClarification bool         `json:"need_clarification,omitempty"`
	Review            ReviewResult `json:"review"`
}

type ExecutionResult struct {
	Review        ReviewResult     `json:"review"`
	Chart         ChartSuggestion  `json:"chart"`
	Answer        string           `json:"answer,omitempty"`
	Error         string           `json:"error,omitempty"`
	Columns       []string         `json:"columns,omitempty"`
	Rows          []map[string]any `json:"rows,omitempty"`
	SpeechSummary string           `json:"speech_summary,omitempty"`
}

type ReviewResult struct {
	Passed        bool     `json:"passed"`
	RiskLevel     string   `json:"risk_level"`
	NormalizedSQL string   `json:"normalized_sql,omitempty"`
	BlockedReason string   `json:"blocked_reason,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	Limit         int      `json:"limit,omitempty"`
}

type ChartSuggestion struct {
	Type       string   `json:"type"`
	XField     string   `json:"x_field,omitempty"`
	YFields    []string `json:"y_fields,omitempty"`
	NameField  string   `json:"name_field,omitempty"`
	ValueField string   `json:"value_field,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

type StreamEvent struct {
	Name string
	Data json.RawMessage
}

func NewClient(baseURL, appID, appSecret string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		AppID:     strings.TrimSpace(appID),
		AppSecret: strings.TrimSpace(appSecret),
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ServerChatData, error) {
	var out Envelope[ServerChatData]
	if err := c.postJSON(ctx, "/embed/server/chat", req, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("ling-shu error code=%d message=%s request_id=%s", out.Code, out.Message, out.RequestID)
	}
	return &out.Data, nil
}

func (c *Client) StreamChat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent) error) (*ServerChatData, error) {
	payload, err := json.Marshal(c.payload(req))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embed/server/chat/stream", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	c.setAuthHeaders(httpReq, req.Identity)

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readError(resp)
	}

	var final *ServerChatData
	if err := readSSE(resp.Body, func(event StreamEvent) error {
		if onEvent != nil {
			if err := onEvent(event); err != nil {
				return err
			}
		}
		if event.Name == "result" {
			var result ServerChatData
			if err := json.Unmarshal(event.Data, &result); err != nil {
				return err
			}
			final = &result
		}
		if event.Name == "error" {
			var body struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(event.Data, &body)
			if body.Message == "" {
				body.Message = string(event.Data)
			}
			return errors.New(body.Message)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if final == nil {
		return nil, errors.New("stream finished without result")
	}
	return final, nil
}

func (c *Client) postJSON(ctx context.Context, path string, req ChatRequest, out any) error {
	payload, err := json.Marshal(c.payload(req))
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpReq, req.Identity)

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) payload(req ChatRequest) chatPayload {
	return chatPayload{
		Key:         strings.TrimSpace(req.Key),
		Content:     strings.TrimSpace(req.Content),
		MaxRows:     req.MaxRows,
		AutoExecute: req.AutoExecute,
	}
}

func (c *Client) setAuthHeaders(req *http.Request, identity Identity) {
	req.Header.Set("X-Ling-Shu-App-Id", c.AppID)
	req.Header.Set("X-Ling-Shu-App-Secret", c.AppSecret)
	req.Header.Set("X-Ling-Shu-External-User-Id", strings.TrimSpace(identity.ExternalUserID))
	if strings.TrimSpace(identity.ExternalUserName) != "" {
		req.Header.Set("X-Ling-Shu-External-User-Name", strings.TrimSpace(identity.ExternalUserName))
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func readError(resp *http.Response) error {
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var body Envelope[json.RawMessage]
	if err := json.Unmarshal(content, &body); err == nil && body.Message != "" {
		return fmt.Errorf("ling-shu http=%d code=%d message=%s request_id=%s", resp.StatusCode, body.Code, body.Message, body.RequestID)
	}
	return fmt.Errorf("ling-shu http=%d body=%s", resp.StatusCode, strings.TrimSpace(string(content)))
}

func readSSE(r io.Reader, onEvent func(StreamEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	eventName := "message"
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = "message"
			return nil
		}
		event := StreamEvent{
			Name: eventName,
			Data: json.RawMessage(strings.Join(dataLines, "\n")),
		}
		eventName = "message"
		dataLines = nil
		if onEvent != nil {
			return onEvent(event)
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			eventName = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func main() {
	cfg := struct {
		BaseURL          string
		AppID            string
		AppSecret        string
		ExternalUserID   string
		ExternalUserName string
		SessionKey       string
		Question         string
		MaxRows          int
		Stream           bool
	}{
		BaseURL:          env("LINGSHU_API_BASE_URL", "http://localhost:8080/api/v1"),
		AppID:            env("LINGSHU_EMBED_APP_ID", "emb_Shvl2HMCMtMLNfZ-OdVH"),
		AppSecret:        env("LINGSHU_EMBED_APP_SECRET", "lsk_kyzlGzX4oj1V3zQIo0VCExJXGBfTaNOmegpazQrjQUQ"),
		ExternalUserID:   env("DEMO_EXTERNAL_USER_ID", "third-party-user-001"),
		ExternalUserName: env("DEMO_EXTERNAL_USER_NAME", "三方系统测试用户"),
		SessionKey:       env("DEMO_SESSION_KEY", "dashboard:demo"),
		Question:         env("DEMO_QUESTION", "甬舟铁路平均每日掘进多少环?"),
		MaxRows:          envInt("DEMO_MAX_ROWS", 200),
		Stream:           envBool("DEMO_STREAM", true),
	}
	if cfg.AppID == "" || cfg.AppSecret == "" {
		exitf("请先设置 LINGSHU_EMBED_APP_ID 和 LINGSHU_EMBED_APP_SECRET")
	}

	client := NewClient(cfg.BaseURL, cfg.AppID, cfg.AppSecret)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req := ChatRequest{
		Identity: Identity{
			ExternalUserID:   cfg.ExternalUserID,
			ExternalUserName: cfg.ExternalUserName,
		},
		Key:     cfg.SessionKey,
		Content: cfg.Question,
		MaxRows: cfg.MaxRows,
	}

	var (
		result *ServerChatData
		err    error
	)
	if cfg.Stream {
		result, err = client.StreamChat(ctx, req, func(event StreamEvent) error {
			if event.Name == "result" || event.Name == "session" {
				return nil
			}
			fmt.Printf("[stream] %s %s\n", event.Name, compactJSON(event.Data))
			return nil
		})
	} else {
		result, err = client.Chat(ctx, req)
	}
	if err != nil {
		exitf("问数失败：%v", err)
	}
	printResult(result)
}

func printResult(result *ServerChatData) {
	fmt.Printf("session_id: %d\n", result.Session.SessionID)
	fmt.Printf("session_key: %s\n", result.Session.SessionKey)

	answer := result.Result.Agent.Answer
	if result.Result.Execution != nil && strings.TrimSpace(result.Result.Execution.Answer) != "" {
		answer = result.Result.Execution.Answer
	}
	fmt.Printf("answer: %s\n", strings.TrimSpace(answer))
	if result.Result.Agent.SQL != "" {
		fmt.Printf("sql: %s\n", result.Result.Agent.SQL)
	}
	if result.Result.Execution == nil {
		return
	}
	fmt.Printf("chart: %s\n", result.Result.Execution.Chart.Type)
	if len(result.Result.Execution.Rows) == 0 {
		return
	}
	preview, _ := json.MarshalIndent(result.Result.Execution.Rows, "", "  ")
	fmt.Printf("rows:\n%s\n", preview)
}

func compactJSON(raw json.RawMessage) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	content, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(content)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
