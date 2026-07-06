package subscription

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/taiiok/xiaomi-vless/internal/config"
	"github.com/taiiok/xiaomi-vless/internal/httpclient"
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: httpclient.Default(30 * time.Second),
	}
}

func (f *Fetcher) FetchSubscription(subURL string) ([]config.Node, error) {
	req, err := http.NewRequest(http.MethodGet, subURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Xiaomi-VLESS-Panel/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return ParseSubscriptionBody(body)
}

func StampSubscriptionNodes(nodes []config.Node, sub config.Subscription) []config.Node {
	out := make([]config.Node, len(nodes))
	for i, n := range nodes {
		n.SubscriptionID = sub.ID
		n.UpdatedAt = time.Now()
		out[i] = n
	}
	return out
}

func NewSubscription(name, subURL string) config.Subscription {
	return config.Subscription{
		ID:        uuid.NewString(),
		Name:      name,
		URL:       subURL,
		UpdatedAt: time.Now(),
	}
}
