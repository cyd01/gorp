package routing

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type requestLogContextKey struct{}

type requestLogDetails struct {
	route   string
	backend string
}

// SetRequestLogMetadata adds route selection details for an enclosing logger.
func SetRequestLogMetadata(r *http.Request, route, backend string) {
	details, ok := r.Context().Value(requestLogContextKey{}).(*requestLogDetails)
	if !ok {
		details = &requestLogDetails{}
		*r = *r.WithContext(context.WithValue(r.Context(), requestLogContextKey{}, details))
	}
	details.route = route
	details.backend = backend
}

// Logging records request details without inspecting or buffering the body.
func (bm *BuiltinMiddleware) Logging() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			details, ok := r.Context().Value(requestLogContextKey{}).(*requestLogDetails)
			if !ok {
				details = &requestLogDetails{}
				r = r.WithContext(context.WithValue(r.Context(), requestLogContextKey{}, details))
			}
			response := &loggingResponseWriter{ResponseWriter: w}
			start := time.Now()
			next.ServeHTTP(response, r)
			status := response.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Printf(
				"http_request method=%q uri=%q remote=%q status=%d duration=%q route=%q backend=%q correlation_id=%q",
				r.Method,
				r.RequestURI,
				r.RemoteAddr,
				status,
				time.Since(start).String(),
				details.route,
				details.backend,
				r.Header.Get("X-Correlation-Id"),
			)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *loggingResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *loggingResponseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *loggingResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}
