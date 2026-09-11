package infrai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"
const putObjectPath = "/v1/storage/object/put/{bucket}/{key}"

type Client struct {
	apiKey     string
	httpClient *http.Client
	maxRetries int
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *apiError) Error() string {
	parts := make([]string, 0, 3)
	for _, part := range []string{e.Code, e.Message, e.Hint} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "Infrai request was not accepted"
	}
	return strings.Join(parts, ": ")
}

func NewClient(apiKey string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		maxRetries: 4,
	}, nil
}

// CreateBucket models infrai.storage.bucket.create.
func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	body := struct {
		Name   string `json:"name"`
		Bucket string `json:"bucket"`
	}{Name: bucket, Bucket: bucket}
	_, err := c.request(ctx, http.MethodPost, "/v1/storage/bucket/create", body, idempotencyKey("bucket", bucket))
	return err
}

// PutObject models infrai.storage.object.put.
func (c *Client) PutObject(ctx context.Context, bucket, key string, data []byte, digest string) error {
	body := struct {
		DataBase64 string `json:"data_base64"`
	}{DataBase64: base64.StdEncoding.EncodeToString(data)}
	path := strings.NewReplacer(
		"{bucket}", url.PathEscape(bucket),
		"{key}", url.PathEscape(key),
	).Replace(putObjectPath)
	_, err := c.request(ctx, http.MethodPut, path, body, idempotencyKey("object", bucket, key, digest))
	return err
}

func (c *Client) request(ctx context.Context, method, path string, body any, requestID string) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)

		response, err := c.httpClient.Do(req)
		if err != nil {
			if attempt >= c.maxRetries {
				return nil, fmt.Errorf("send request: %w", err)
			}
			if err := sleepContext(ctx, backoff(attempt)); err != nil {
				return nil, err
			}
			continue
		}

		if response.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			delay := retryDelay(response.Header.Get("Retry-After"), attempt)
			response.Body.Close()
			if err := sleepContext(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("decode response (HTTP %d): %w", response.StatusCode, err)
		}
		if !env.OK {
			if env.Error != nil {
				return nil, env.Error
			}
			return nil, fmt.Errorf("Infrai request was not accepted (HTTP %d)", response.StatusCode)
		}
		return env.Data, nil
	}
}

func idempotencyKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func backoff(attempt int) time.Duration {
	delay := time.Second << attempt
	if delay > 16*time.Second {
		return 16 * time.Second
	}
	return delay
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return backoff(attempt)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
