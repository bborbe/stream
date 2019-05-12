package cache

import (
	"context"
	"net/http"
)

type Connection interface {
	Response() *http.Response
	Stream() Stream
}

func NewConnection(
	ctx context.Context,
	resp *http.Response,
) Connection {
	return &connection{
		resp: resp,
		stream: NewStream(
			ctx,
			resp.Body,
		),
	}
}

type connection struct {
	resp   *http.Response
	stream Stream
}

func (c *connection) Stream() Stream {
	return c.stream
}

func (c *connection) Response() *http.Response {
	result := *c.resp
	result.Body = c.stream.CreateListener()
	return &result
}
