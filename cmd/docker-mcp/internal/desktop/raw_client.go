package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"
)

var ClientBackend = newRawClient(dialBackend)

func AvoidResourceSaverMode(ctx context.Context) {
	_ = ClientBackend.Post(ctx, "/idle/make-busy", nil, nil)
}

type RawClient struct {
	client  func() *http.Client
	timeout time.Duration
}

func newRawClient(dialer func(ctx context.Context) (net.Conn, error)) *RawClient {
	return &RawClient{
		client: func() *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (conn net.Conn, err error) {
						return dialer(ctx)
					},
				},
			}
		},
		timeout: 10 * time.Second,
	}
}

func (c *RawClient) Get(ctx context.Context, endpoint string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost"+endpoint, http.NoBody)
	if err != nil {
		return err
	}

	response, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(buf, &v); err != nil {
		return err
	}
	return nil
}

func (c *RawClient) Post(ctx context.Context, endpoint string, v any, result any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost"+endpoint, bytes.NewReader(buf))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if result == nil {
		_, err := io.Copy(io.Discard, response.Body)
		return err
	}

	buf, err = io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(buf, result); err != nil {
		return err
	}
	return nil
}

func (c *RawClient) Delete(ctx context.Context, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://localhost"+endpoint, http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	_, err = io.Copy(io.Discard, response.Body)
	return err
}
