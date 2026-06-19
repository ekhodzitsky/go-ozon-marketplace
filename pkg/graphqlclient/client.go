// Package graphqlclient — минимальный GraphQL HTTP-клиент.
package graphqlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client выполняет GraphQL-запросы по HTTP.
type Client struct {
	URL        string
	HTTPClient *http.Client
}

// NewClient создаёт клиент для заданного URL с таймаутом по умолчанию.
func NewClient(url string) *Client {
	return &Client{
		URL:        url,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Do выполняет GraphQL-запрос и распаковывает данные ответа в result.
// result может быть nil, если вызывающему нужно только проверить ошибки.
func (c *Client) Do(ctx context.Context, query string, result interface{}) error {
	body, err := json.Marshal(request{Query: query})
	if err != nil {
		return fmt.Errorf("marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql returned status %d", resp.StatusCode)
	}

	var gr response
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return fmt.Errorf("graphql decode failed: %w", err)
	}

	if len(gr.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", joinErrors(gr.Errors))
	}

	if result != nil {
		if err := json.Unmarshal(gr.Data, result); err != nil {
			return fmt.Errorf("graphql unmarshal data: %w", err)
		}
	}

	return nil
}

type request struct {
	Query string `json:"query"`
}

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors"`
}

type graphqlError struct {
	Message string `json:"message"`
}

func joinErrors(errs []graphqlError) string {
	messages := make([]string, len(errs))
	for i, e := range errs {
		messages[i] = e.Message
	}
	return strings.Join(messages, "; ")
}
