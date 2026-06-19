package nls

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCreateTokenSignatureMatchesAliyunExample(t *testing.T) {
	params := map[string]string{
		"AccessKeyId":      "my_access_key_id",
		"Action":           "CreateToken",
		"Version":          "2019-02-28",
		"Timestamp":        "2019-04-18T08:32:31Z",
		"Format":           "JSON",
		"RegionId":         "cn-shanghai",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   "b924c8c3-6d03-4c5d-ad36-d984d3116788",
	}

	query := canonicalizedQuery(params)
	if !strings.Contains(query, "Timestamp=2019-04-18T08%3A32%3A31Z") {
		t.Fatalf("timestamp should be percent encoded: %s", query)
	}
	signature := sign("GET", "/", query, "my_access_key_secret")
	if signature != "hHq4yNsPitlfDJ2L0nQPdugdEzM=" {
		t.Fatalf("unexpected signature: %s", signature)
	}
}

func TestOpenAPITokenProviderFetchesAndCachesToken(t *testing.T) {
	requests := 0
	provider := NewOpenAPITokenProvider(TokenProviderConfig{
		AccessKeyID:     "ak-id",
		AccessKeySecret: "ak-secret",
		Endpoint:        "https://nls-meta.cn-shanghai.aliyuncs.com/",
		RegionID:        "cn-shanghai",
		RefreshBefore:   time.Minute,
	})
	provider.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		values := r.URL.Query()
		if values.Get("Action") != "CreateToken" {
			t.Fatalf("expected CreateToken action")
		}
		if values.Get("Signature") == "" {
			t.Fatalf("expected signature")
		}
		body, err := json.Marshal(map[string]any{
			"NlsRequestId": "nls-request-id",
			"RequestId":    "request-id",
			"ErrMsg":       "",
			"Token": map[string]any{
				"Id":         "token-id",
				"ExpireTime": time.Now().Add(time.Hour).Unix(),
				"UserId":     "user-id",
			},
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    r,
		}, nil
	})}
	first, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	second, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("get cached token: %v", err)
	}
	if first != "token-id" || second != "token-id" {
		t.Fatalf("unexpected token values: %s %s", first, second)
	}
	if requests != 1 {
		t.Fatalf("expected one token request, got %d", requests)
	}
}

func TestSignedURLKeepsEndpointAndAddsSignature(t *testing.T) {
	rawURL, err := signedURL("https://nls-meta.cn-shanghai.aliyuncs.com/", map[string]string{
		"AccessKeyId":      "ak-id",
		"Action":           "CreateToken",
		"Version":          "2019-02-28",
		"Format":           "JSON",
		"RegionId":         "cn-shanghai",
		"Timestamp":        "2026-06-15T00:00:00Z",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   "b924c8c3-6d03-4c5d-ad36-d984d3116788",
	}, "ak-secret")
	if err != nil {
		t.Fatalf("signed url: %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse signed url: %v", err)
	}
	if parsed.Host != "nls-meta.cn-shanghai.aliyuncs.com" {
		t.Fatalf("unexpected host: %s", parsed.Host)
	}
	if parsed.Query().Get("Signature") == "" {
		t.Fatalf("expected signature query")
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
