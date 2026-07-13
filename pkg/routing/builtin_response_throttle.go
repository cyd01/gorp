package routing

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/cyd01/gorp/pkg/backend"
)

// ThrottleResponse limits the rate at which a response body is read.
func (bm *BuiltinMiddleware) ThrottleResponse(bytesPerSecond int64) backend.ModifyResponse {
	return func(response *http.Response) error {
		if response == nil || response.Body == nil {
			return nil
		}

		ctx := context.Background()
		if response.Request != nil {
			ctx = response.Request.Context()
		}
		response.Body = &throttledResponseBody{
			ReadCloser:     response.Body,
			bytesPerSecond: bytesPerSecond,
			ctx:            ctx,
		}
		return nil
	}
}

type throttledResponseBody struct {
	io.ReadCloser
	bytesPerSecond int64
	ctx            context.Context
}

func (body *throttledResponseBody) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return body.ReadCloser.Read(buffer)
	}

	maxChunk := body.bytesPerSecond / 10
	if body.bytesPerSecond%10 != 0 {
		maxChunk++
	}
	if maxChunk < 1 {
		maxChunk = 1
	}
	if maxChunk < int64(len(buffer)) {
		buffer = buffer[:int(maxChunk)]
	}

	n, err := body.ReadCloser.Read(buffer)
	if n == 0 || body.bytesPerSecond <= 0 {
		return n, err
	}

	delayNanos := float64(n) * float64(time.Second) / float64(body.bytesPerSecond)
	if delayNanos <= 0 {
		return n, err
	}
	maxDuration := time.Duration(1<<63 - 1)
	if delayNanos >= float64(1<<63) {
		delayNanos = float64(maxDuration)
	}

	timer := time.NewTimer(time.Duration(delayNanos))
	defer timer.Stop()
	select {
	case <-body.ctx.Done():
		return 0, body.ctx.Err()
	case <-timer.C:
		return n, err
	}
}
