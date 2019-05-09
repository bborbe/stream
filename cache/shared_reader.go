package cache

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type SharedReadCloser interface {
	ReadCloser() io.ReadCloser
	Close() error
	Counter() int
}

func NewSharedReadCloser(
	ctx context.Context,
	reader io.ReadCloser,
) SharedReadCloser {
	ctx, cancel := context.WithCancel(ctx)
	return &sharedReadCloser{
		ctx:     ctx,
		cancel:  cancel,
		reader:  reader,
		content: make([]byte, 0),
		done:    make(chan struct{}),
	}
}

type sharedReadCloser struct {
	ctx     context.Context
	cancel  context.CancelFunc
	reader  io.ReadCloser
	once    sync.Once
	counter int
	done    chan struct{}

	mux     sync.Mutex
	content []byte
	err     error
}

func (s *sharedReadCloser) ReadCloser() io.ReadCloser {
	s.once.Do(func() {
		go func() {
			defer close(s.done)
			defer s.reader.Close()
			for {
				select {
				case <-s.ctx.Done():
					return
				default:
					buf := make([]byte, 1024)
					n, err := s.reader.Read(buf)
					if err != nil {
						s.mux.Lock()
						s.err = err
						s.mux.Unlock()
						return
					}
					s.mux.Lock()
					s.content = append(s.content, buf[:n]...)
					s.mux.Unlock()
					glog.V(2).Infof("cache size %d", len(s.content))
				}
			}
		}()
	})
	s.counter++
	return &readCloser{
		pos:              0,
		sharedReadCloser: s,
	}
}

func (s *sharedReadCloser) Counter() int {
	return s.counter
}

func (s *sharedReadCloser) Close() error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-time.After(time.Second):
		return errors.New("closed timeout")
	}
}

type readCloser struct {
	pos              int
	sharedReadCloser *sharedReadCloser

	mux    sync.Mutex
	closed bool
}

func (r *readCloser) Read(p []byte) (int, error) {
	r.sharedReadCloser.mux.Lock()
	defer r.sharedReadCloser.mux.Unlock()
	n := copy(p, r.sharedReadCloser.content[r.pos:])
	r.pos += n
	if n == 0 {
		if r.sharedReadCloser.err != nil {
			return 0, r.sharedReadCloser.err
		}
	}
	return n, nil
}

func (r *readCloser) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.closed {
		return nil
	}
	r.sharedReadCloser.counter--
	r.closed = true
	return nil
}
