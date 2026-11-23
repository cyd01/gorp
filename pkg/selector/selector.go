package selector

import (
	"errors"
	"github.com/cyd01/gorp/pkg/backend"
	"net/http"
	"time"
)

type Selector interface {
	Select(*http.Request) (*backend.Backend, error)
}

type ObservableSelector interface {
	Selector
	OnSuccess(*backend.Backend, time.Duration)
	OnFailure(*backend.Backend, error)
}

var ErrNoBackend = errors.New("no backend available")
var ErrNoValidBackend = errors.New("no valid backend")
