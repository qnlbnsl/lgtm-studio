package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

var ErrLinkFrameTooLarge = errors.New("The link frame is too large.")

type linkStreamKey struct{}

type linkStream struct {
	body   *linkBody
	writer *linkWriter
}

// WithLinkStream bounds request frames and exposes the controls for a link stream.
func WithLinkStream(next http.Handler, maxFrameBytes int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		stream := &linkStream{
			body:   &linkBody{ReadCloser: r.Body, limit: maxFrameBytes},
			writer: &linkWriter{ResponseWriter: w},
		}
		r = r.WithContext(context.WithValue(r.Context(), linkStreamKey{}, stream))
		r.Body = stream.body
		next.ServeHTTP(stream.writer, r)
	})
}

type linkBody struct {
	io.ReadCloser
	limit     int
	prefix    [5]byte
	unread    []byte
	remaining int64
	prefixErr error
	close     sync.Once
	closeErr  error
}

func (b *linkBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(b.unread) == 0 && b.remaining == 0 {
		if b.prefixErr != nil {
			return 0, b.prefixErr
		}
		if _, err := io.ReadFull(b.ReadCloser, b.prefix[:]); err != nil {
			b.prefixErr = err
			return 0, err
		}
		size := int64(binary.BigEndian.Uint32(b.prefix[1:]))
		if b.limit > 0 && size > int64(b.limit) {
			b.prefixErr = ErrLinkFrameTooLarge
			return 0, b.prefixErr
		}
		b.remaining = size
		b.unread = b.prefix[:]
	}
	if len(b.unread) > 0 {
		n := copy(p, b.unread)
		b.unread = b.unread[n:]
		return n, nil
	}
	n, err := b.ReadCloser.Read(p[:min(int64(len(p)), b.remaining)])
	b.remaining -= int64(n)
	return n, err
}

func (b *linkBody) Close() error {
	b.close.Do(func() { b.closeErr = b.ReadCloser.Close() })
	return b.closeErr
}

// CloseLinkRead unblocks an open receive while leaving the response writable.
func CloseLinkRead(ctx context.Context) error {
	stream, ok := ctx.Value(linkStreamKey{}).(*linkStream)
	if !ok {
		return http.ErrNotSupported
	}
	return stream.body.Close()
}

// BoundLinkEnd sets the deadline for the protocol's terminal trailers or envelope.
func BoundLinkEnd(ctx context.Context, limit time.Duration) {
	if deadline, ok := ctx.Value(deadlineKey{}).(writeDeadline); ok {
		_ = deadline.set(time.Now().Add(limit))
	}
}

type linkWriter struct {
	http.ResponseWriter
	welcome *welcomeWrite
}

func (w *linkWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *linkWriter) Write(p []byte) (int, error) {
	if permit := w.welcome; permit != nil {
		if len(p) == 0 {
			return 0, permit.err
		}
		if !permit.started && permit.err == nil {
			permit.start()
		}
		if permit.err != nil {
			return 0, permit.err
		}
	}
	return w.ResponseWriter.Write(p)
}

func (w *linkWriter) Flush() { _ = w.FlushError() }

func (w *linkWriter) FlushError() error {
	permit := w.welcome
	if permit != nil && (!permit.started || permit.err != nil) {
		return permit.err
	}
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if permit != nil && permit.flushErr == nil {
		permit.flushErr = err
	}
	return err
}

type welcomeWrite struct {
	ctx      context.Context
	begin    func() error
	deadline writeDeadline
	started  bool
	err      error
	flushErr error
	stop     func() bool
	done     chan struct{}
}

func (p *welcomeWrite) start() {
	if p.err = p.begin(); p.err != nil {
		return
	}
	if p.err = p.deadline.set(time.Now().Add(p.deadline.timeout)); p.err != nil {
		return
	}
	p.started = true
	p.done = make(chan struct{})
	p.stop = context.AfterFunc(p.ctx, func() {
		defer close(p.done)
		_ = p.deadline.set(time.Now())
	})
}

// SendLinkWelcome authorizes the first frame at the write boundary.
func SendLinkWelcome(ctx context.Context, begin func() error, send func() error) (err error) {
	stream, ok := ctx.Value(linkStreamKey{}).(*linkStream)
	deadline, hasDeadline := ctx.Value(deadlineKey{}).(writeDeadline)
	if !ok || !hasDeadline {
		return ErrNoDeadline
	}
	permit := &welcomeWrite{ctx: ctx, begin: begin, deadline: deadline}
	stream.writer.welcome = permit
	defer func() {
		stream.writer.welcome = nil
		if permit.err != nil {
			err = permit.err
		} else if err == nil {
			err = permit.flushErr
		}
		if permit.stop == nil {
			return
		}
		if !permit.stop() {
			<-permit.done
			return
		}
		if clearErr := deadline.set(time.Time{}); err == nil {
			err = clearErr
		}
	}()
	return send()
}
