package cache

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type Stream interface {
	Close() error
	CreateListener() Listener
	Listeners() []Listener
	Size() int
	Lock()
	Unlock()
	Content() []byte
	Error() error
	AddListener(listener Listener)
	RemoveListener(listener Listener)
}

func NewStream(
	ctx context.Context,
	reader io.ReadCloser,
) Stream {
	ctx, cancel := context.WithCancel(ctx)
	return &stream{
		ctx:     ctx,
		cancel:  cancel,
		reader:  reader,
		content: make([]byte, 0),
		done:    make(chan struct{}),
	}
}

type stream struct {
	ctx    context.Context
	cancel context.CancelFunc
	reader io.ReadCloser
	once   sync.Once
	done   chan struct{}

	mux       sync.Mutex
	content   []byte
	err       error
	listeners []Listener
}

func (s *stream) Content() []byte {
	return s.content
}

func (s *stream) Lock() {
	//s.mux.Lock()
}

func (s *stream) Unlock() {
	//s.mux.Unlock()
}

func (s *stream) Error() error {
	return s.err
}

func (s *stream) CreateListener() Listener {
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
						s.Lock()
						s.err = err
						s.Unlock()
						return
					}
					s.Lock()
					s.content = append(s.content, buf[:n]...)
					s.Unlock()
					glog.V(2).Infof("cache size %d", len(s.content))
				}
			}
		}()
	})
	listener := NewListener(s)
	s.AddListener(listener)
	return listener
}

func (s *stream) AddListener(listener Listener) {
	s.Lock()
	defer s.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *stream) RemoveListener(listener Listener) {
	s.Lock()
	defer s.Unlock()
	var listeners []Listener
	for _, l := range s.listeners {
		if l != listener {
			listeners = append(listeners, l)
		}

	}
	s.listeners = listeners
}

func (s *stream) Listeners() []Listener {
	return s.listeners
}

func (s *stream) Size() int {
	s.Lock()
	defer s.Unlock()
	return len(s.content)
}

func (s *stream) Close() error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-time.After(time.Second):
		return errors.New("closed timeout")
	}
}

type Listener interface {
	io.ReadCloser
	Position() int
	SetPosition(pos int)
}

func NewListener(stream Stream) Listener {
	return &listener{
		pos:    0,
		stream: stream,
	}
}

type listener struct {
	stream Stream

	pos    int
	closed bool
}

func (l *listener) SetPosition(pos int) {
	l.stream.Lock()
	defer l.stream.Unlock()
	l.pos = pos
}
func (l *listener) Position() int {
	l.stream.Lock()
	defer l.stream.Unlock()
	return l.pos
}

func (l *listener) Read(p []byte) (int, error) {
	l.stream.Lock()
	defer l.stream.Unlock()
	n := copy(p, l.stream.Content()[l.pos:])
	l.pos += n
	if n == 0 {
		if l.stream.Error() != nil {
			return 0, l.stream.Error()
		}
	}
	return n, nil
}

func (l *listener) Close() error {
	l.stream.Lock()
	defer l.stream.Unlock()

	if l.closed {
		return nil
	}
	l.stream.RemoveListener(l)
	l.closed = true
	return nil
}
