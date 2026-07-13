package routing

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cyd01/gorp/pkg/backend"
)

// AddResponseHeaders adds headers to the response.
func (bm *BuiltinMiddleware) AddResponseHeaders(headers map[string]string) backend.ModifyResponse {
	return func(resp *http.Response) error {
		for key, value := range headers {
			resp.Header.Add(key, value)
		}
		return nil
	}
}

// SetResponseHeaders replaces headers in the response.
func (bm *BuiltinMiddleware) SetResponseHeaders(headers map[string]string) backend.ModifyResponse {
	return func(resp *http.Response) error {
		for key, value := range headers {
			resp.Header.Set(key, value)
		}
		return nil
	}
}

// ModifyResponseHeaders replaces headers in the response.
func (bm *BuiltinMiddleware) ModifyResponseHeaders(headers map[string]string) backend.ModifyResponse {
	return func(resp *http.Response) error {
		for key, value := range headers {
			resp.Header.Set(key, value)
		}
		return nil
	}
}

// RemoveResponseHeaders removes headers from the response.
func (bm *BuiltinMiddleware) RemoveResponseHeaders(headerNames []string) backend.ModifyResponse {
	return func(resp *http.Response) error {
		for _, name := range headerNames {
			resp.Header.Del(name)
		}
		return nil
	}
}

// AddResponseLocationPrefix adds a prefix to the Location header path.
func (bm *BuiltinMiddleware) AddResponseLocationPrefix(prefix string) backend.ModifyResponse {
	return func(resp *http.Response) error {
		if resp == nil {
			return nil
		}

		location := resp.Header.Get("Location")
		if location == "" {
			return nil
		}

		u, err := url.Parse(location)
		if err != nil {
			return err
		}
		u.Path = prefixLocationPath(prefix, u.Path)
		u.RawPath = ""
		resp.Header.Set("Location", u.String())
		return nil
	}
}

// SetResponseLocationHost replaces the host of an absolute HTTP(S) Location.
func (bm *BuiltinMiddleware) SetResponseLocationHost(host string) backend.ModifyResponse {
	return func(resp *http.Response) error {
		if resp == nil {
			return nil
		}

		location := resp.Header.Get("Location")
		if location == "" {
			return nil
		}

		u, err := url.Parse(location)
		if err != nil {
			return err
		}
		if u.Host == "" {
			return nil
		}

		u.Host = host
		resp.Header.Set("Location", u.String())
		return nil
	}
}

func prefixLocationPath(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" {
		return prefix
	}
	if prefix == "/" {
		return "/" + strings.TrimLeft(path, "/")
	}
	return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
}
