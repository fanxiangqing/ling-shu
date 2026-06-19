package nls

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultTokenEndpoint      = "https://nls-meta.cn-shanghai.aliyuncs.com/"
	DefaultTokenRegionID      = "cn-shanghai"
	DefaultTokenRefreshBefore = 10 * time.Minute
)

var ErrTokenCredentialsNotConfigured = errors.New("aliyun nls token credentials are not configured")

type TokenProvider interface {
	Configured() bool
	Token(ctx context.Context) (string, error)
}

type TokenProviderConfig struct {
	StaticToken     string
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	RegionID        string
	RefreshBefore   time.Duration
	Timeout         time.Duration
}

type StaticTokenProvider struct {
	token string
}

func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: strings.TrimSpace(token)}
}

func (p *StaticTokenProvider) Configured() bool {
	return p != nil && p.token != ""
}

func (p *StaticTokenProvider) Token(ctx context.Context) (string, error) {
	if !p.Configured() {
		return "", ErrTokenCredentialsNotConfigured
	}
	return p.token, nil
}

type OpenAPITokenProvider struct {
	accessKeyID     string
	accessKeySecret string
	endpoint        string
	regionID        string
	refreshBefore   time.Duration
	client          *http.Client

	mu         sync.Mutex
	token      string
	expireTime time.Time
}

func NewTokenProvider(cfg TokenProviderConfig) TokenProvider {
	if strings.TrimSpace(cfg.StaticToken) != "" {
		return NewStaticTokenProvider(cfg.StaticToken)
	}
	return NewOpenAPITokenProvider(cfg)
}

func NewOpenAPITokenProvider(cfg TokenProviderConfig) *OpenAPITokenProvider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultTokenEndpoint
	}
	regionID := cfg.RegionID
	if regionID == "" {
		regionID = DefaultTokenRegionID
	}
	refreshBefore := cfg.RefreshBefore
	if refreshBefore <= 0 {
		refreshBefore = DefaultTokenRefreshBefore
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &OpenAPITokenProvider{
		accessKeyID:     strings.TrimSpace(cfg.AccessKeyID),
		accessKeySecret: strings.TrimSpace(cfg.AccessKeySecret),
		endpoint:        endpoint,
		regionID:        regionID,
		refreshBefore:   refreshBefore,
		client:          &http.Client{Timeout: timeout},
	}
}

func (p *OpenAPITokenProvider) Configured() bool {
	return p != nil && p.accessKeyID != "" && p.accessKeySecret != ""
}

func (p *OpenAPITokenProvider) Token(ctx context.Context) (string, error) {
	if !p.Configured() {
		return "", ErrTokenCredentialsNotConfigured
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && time.Now().Add(p.refreshBefore).Before(p.expireTime) {
		return p.token, nil
	}

	token, expireTime, err := p.createToken(ctx)
	if err != nil {
		return "", err
	}
	p.token = token
	p.expireTime = expireTime
	return p.token, nil
}

func (p *OpenAPITokenProvider) createToken(ctx context.Context) (string, time.Time, error) {
	params := map[string]string{
		"AccessKeyId":      p.accessKeyID,
		"Action":           "CreateToken",
		"Version":          "2019-02-28",
		"Format":           "JSON",
		"RegionId":         p.regionID,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   newUUID(),
	}
	requestURL, err := signedURL(p.endpoint, params, p.accessKeySecret)
	if err != nil {
		return "", time.Time{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create aliyun nls token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("request aliyun nls token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read aliyun nls token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr tokenErrorResponse
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Message != "" || apiErr.Code != "" {
			return "", time.Time{}, fmt.Errorf("aliyun nls token api error: status=%d code=%s message=%s", resp.StatusCode, apiErr.Code, apiErr.Message)
		}
		return "", time.Time{}, fmt.Errorf("aliyun nls token api error: status=%d", resp.StatusCode)
	}

	var result createTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("decode aliyun nls token response: %w", err)
	}
	if result.ErrMsg != "" {
		return "", time.Time{}, errors.New("aliyun nls token api error: " + result.ErrMsg)
	}
	if result.Token.ID == "" {
		return "", time.Time{}, errors.New("aliyun nls token api returned empty token")
	}
	if result.Token.ExpireTime <= 0 {
		return "", time.Time{}, errors.New("aliyun nls token api returned invalid expire_time")
	}
	return result.Token.ID, time.Unix(result.Token.ExpireTime, 0), nil
}

func signedURL(endpoint string, params map[string]string, accessKeySecret string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse aliyun nls token endpoint: %w", err)
	}
	canonicalQuery := canonicalizedQuery(params)
	signature := sign("GET", "/", canonicalQuery, accessKeySecret)
	parsed.RawQuery = "Signature=" + percentEncode(signature) + "&" + canonicalQuery
	return parsed.String(), nil
}

func canonicalizedQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, percentEncode(key)+"="+percentEncode(params[key]))
	}
	return strings.Join(parts, "&")
}

func sign(method string, path string, canonicalQuery string, accessKeySecret string) string {
	stringToSign := method + "&" + percentEncode(path) + "&" + percentEncode(canonicalQuery)
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func percentEncode(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "*", "%2A")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return NewID()
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

type createTokenResponse struct {
	NlsRequestID string `json:"NlsRequestId"`
	RequestID    string `json:"RequestId"`
	ErrMsg       string `json:"ErrMsg"`
	Token        struct {
		ID         string `json:"Id"`
		ExpireTime int64  `json:"ExpireTime"`
		UserID     string `json:"UserId"`
	} `json:"Token"`
}

type tokenErrorResponse struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	RequestID string `json:"RequestId"`
}
