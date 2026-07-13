package metrics

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gorp",
			Subsystem: "proxy",
			Name:      "requests_total",
			Help:      "Total number of proxied HTTP requests.",
		},
		[]string{"backend", "method", "status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gorp",
			Subsystem: "proxy",
			Name:      "request_duration_seconds",
			Help:      "Duration of proxied HTTP requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"backend"},
	)
	backendErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gorp",
			Subsystem: "proxy",
			Name:      "backend_errors_total",
			Help:      "Total number of backend errors.",
		},
		[]string{"backend"},
	)
	activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "gorp",
			Subsystem: "proxy",
			Name:      "active_connections",
			Help:      "Current number of active proxied connections.",
		},
		[]string{"backend"},
	)
	listenerRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gorp",
			Subsystem: "listener",
			Name:      "requests_total",
			Help:      "Total number of requests received by listeners.",
		},
		[]string{"listener", "type", "method", "status"},
	)
	listenerRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gorp",
			Subsystem: "listener",
			Name:      "request_duration_seconds",
			Help:      "Duration of requests handled by listeners in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"listener", "type"},
	)
	listenerActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "gorp",
			Subsystem: "listener",
			Name:      "active_connections",
			Help:      "Current number of active connections handled by listeners.",
		},
		[]string{"listener", "type"},
	)
)

func init() {
	registry.MustRegister(requestsTotal)
	registry.MustRegister(requestDuration)
	registry.MustRegister(backendErrors)
	registry.MustRegister(activeConnections)
	registry.MustRegister(listenerRequestsTotal)
	registry.MustRegister(listenerRequestDuration)
	registry.MustRegister(listenerActiveConnections)
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// ObserveRequest records the result and duration of a proxied request.
func ObserveRequest(backend, method string, status int, duration time.Duration) {
	requestsTotal.WithLabelValues(backend, method, strconv.Itoa(status)).Inc()
	requestDuration.WithLabelValues(backend).Observe(duration.Seconds())
}

// ObserveBackendError records a backend communication error.
func ObserveBackendError(backend string) {
	backendErrors.WithLabelValues(backend).Inc()
}

// SetActiveConnections records the current number of active connections.
func SetActiveConnections(backend string, count int64) {
	activeConnections.WithLabelValues(backend).Set(float64(count))
}

// InstrumentHandler records requests handled by a listener.
func InstrumentHandler(listener, listenerType string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active := listenerActiveConnections.WithLabelValues(listener, listenerType)
		active.Inc()
		start := time.Now()
		responseWriter := &responseWriter{ResponseWriter: w}
		defer func() {
			active.Dec()
			status := responseWriter.status
			if status == 0 {
				status = http.StatusOK
			}
			listenerRequestsTotal.WithLabelValues(listener, listenerType, r.Method, strconv.Itoa(status)).Inc()
			listenerRequestDuration.WithLabelValues(listener, listenerType).Observe(time.Since(start).Seconds())
		}()
		next.ServeHTTP(responseWriter, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *responseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *responseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}
