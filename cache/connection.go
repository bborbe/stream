package cache

import (
	"context"
	"net/http"
)

type Connection interface {
	Response() *http.Response
	SharedReadCloser() SharedReadCloser
}

func NewConnection(
	ctx context.Context,
	resp *http.Response,
) Connection {
	return &connection{
		resp: resp,
		body: NewSharedReadCloser(
			ctx,
			resp.Body,
		),
	}
}

type connection struct {
	resp *http.Response
	body SharedReadCloser
}

func (c *connection) SharedReadCloser() SharedReadCloser {
	return c.body
}

func (c *connection) Response() *http.Response {
	result := *c.resp
	result.Body = c.body.ReadCloser()
	return &result
}
