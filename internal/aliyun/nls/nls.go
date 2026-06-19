package nls

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DefaultWebsocketURL = "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1"
	StatusSuccess       = 20000000
)

type Header struct {
	AppKey        string `json:"appkey,omitempty"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	TaskID        string `json:"task_id,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	Status        int    `json:"status,omitempty"`
	StatusText    string `json:"status_text,omitempty"`
	StatusMessage string `json:"status_message,omitempty"`
}

type OutboundMessage struct {
	Header  Header `json:"header"`
	Payload any    `json:"payload,omitempty"`
}

type InboundMessage struct {
	Header  Header          `json:"header"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Client struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func Dial(ctx context.Context, rawURL string, token string, timeout time.Duration) (*Client, error) {
	if rawURL == "" {
		rawURL = DefaultWebsocketURL
	}
	endpoint, err := withToken(rawURL, token)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	header := http.Header{}
	if strings.TrimSpace(token) != "" {
		header.Set("X-NLS-Token", token)
	}
	dialer := websocket.Dialer{HandshakeTimeout: timeout}
	conn, resp, err := dialer.DialContext(ctx, endpoint, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial aliyun nls websocket: status=%d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("dial aliyun nls websocket: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) WriteJSON(ctx context.Context, message OutboundMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	if err := c.conn.WriteJSON(message); err != nil {
		return fmt.Errorf("write aliyun nls json: %w", err)
	}
	return nil
}

func (c *Client) WriteBinary(ctx context.Context, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write aliyun nls audio: %w", err)
	}
	return nil
}

func (c *Client) ReadFrame(ctx context.Context) (int, []byte, error) {
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetReadDeadline(deadline)
	}
	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, nil, ctxErr
		}
		return 0, nil, fmt.Errorf("read aliyun nls frame: %w", err)
	}
	return messageType, data, nil
}

func NewControlMessage(appKey string, namespace string, name string, taskID string, payload any) OutboundMessage {
	return OutboundMessage{
		Header: Header{
			AppKey:    appKey,
			Namespace: namespace,
			Name:      name,
			TaskID:    taskID,
			MessageID: NewID(),
		},
		Payload: payload,
	}
}

func ParseInbound(data []byte) (*InboundMessage, error) {
	var message InboundMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("parse aliyun nls message: %w", err)
	}
	return &message, nil
}

func HeaderStatusText(header Header) string {
	switch {
	case header.StatusText != "":
		return header.StatusText
	case header.StatusMessage != "":
		return header.StatusMessage
	case header.Status != 0:
		return fmt.Sprintf("%d", header.Status)
	default:
		return ""
	}
}

func HeaderError(header Header) error {
	message := HeaderStatusText(header)
	if message == "" {
		message = header.Name
	}
	if header.Status != 0 {
		return fmt.Errorf("aliyun nls task failed: status=%d message=%s", header.Status, message)
	}
	return errors.New("aliyun nls task failed: " + message)
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return hex.EncodeToString(b[:])
}

func IsTextMessage(messageType int) bool {
	return messageType == websocket.TextMessage
}

func IsBinaryMessage(messageType int) bool {
	return messageType == websocket.BinaryMessage
}

func withToken(rawURL string, token string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse aliyun nls websocket url: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return parsed.String(), nil
	}

	query := parsed.Query()
	if query.Get("token") == "" {
		query.Set("token", token)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
