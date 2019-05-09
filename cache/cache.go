package cache

import (
	"context"
	"net/http"
	"sync"
)

type Cache interface {
	http.RoundTripper
	Connections() map[string]Connection
}

func NewCache(
	ctx context.Context,
	httpClient *http.Client,
) Cache {
	return &cache{
		ctx:        ctx,
		httpClient: httpClient,
		data:       make(map[string]Connection),
	}
}

type cache struct {
	ctx        context.Context
	httpClient *http.Client

	mux  sync.Mutex
	data map[string]Connection
}

func (c *cache) Connections() map[string]Connection {
	return c.data
}

func (c *cache) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	conn, ok := c.data[req.URL.String()]
	if !ok {
		req.RequestURI = ""
		resp, err := c.httpClient.Do(req.WithContext(c.ctx))
		if err != nil {
			return nil, err
		}
		conn = NewConnection(c.ctx, resp)
		c.data[req.URL.String()] = conn
	}

	return conn.Response(), nil
}
