package listener

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// RequestInfo describes an HTTP request.
type RequestInfo struct {
	ServerName       string      `json:"-"`
	ServerVersion    string      `json:"-"`
	ServerHostname   string      `json:"-"`
	StatusCode       int         `json:"-"`
	Server           string      `json:"server"`
	Proto            string      `json:"proto"`
	ProtoMajor       int         `json:"protomajor"`
	ProtoMinor       int         `json:"protominor"`
	Method           string      `json:"method"`
	Host             string      `json:"host"`
	Path             string      `json:"path"`
	URL              *url.URL    `json:"url"`
	Headers          http.Header `json:"headers"`
	Trailers         http.Header `json:"trailers"`
	RemoteAddr       string      `json:"remoteaddr"`
	Close            bool        `json:"close"`
	ContentLength    int64       `json:"contentlength"`
	TransferEncoding []string    `json:"transferencoding"`
	RequestURI       string      `json:"requesturi"`
	Form             url.Values  `json:"form"`
	Body             []byte      `json:"body"`
	BasicAuth        *struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	} `json:"basicauth,omitempty"`
	JWT interface{} `json:"jwt,omitempty"`
	TLS struct {
		Protocol   string `json:"protocol,omitempty"`
		Version    string `json:"version,omitempty"`
		Cipher     string `json:"cipher,omitempty"`
		CipherName string `json:"ciphername,omitempty"`
	} `json:"TLS,omitempty"`
	Hostname struct {
		Name       string `json:"name,omitempty"`
		Pid        int    `json:"pid,omitempty"`
		Ppid       int    `json:"ppid,omitempty"`
		Time       string `json:"time,omitempty"`
		Executable string `json:"executable,omitempty"`
		WorkDir    string `json:"workdir,omitempty"`
	} `json:"hostname"`
}

func (rr *RequestInfo) ReadHostInfo() {
	if h, err := os.Hostname(); err == nil {
		rr.Hostname.Name = h
	}
	if exe, err := os.Executable(); err == nil {
		rr.Hostname.Executable = exe
	}
	if wd, err := os.Getwd(); err == nil {
		rr.Hostname.WorkDir = wd
	}
	rr.Hostname.Pid = os.Getpid()
	rr.Hostname.Ppid = os.Getppid()
	rr.Hostname.Time = time.Now().String()
}
func (rr *RequestInfo) SetServer(name, version string) {
	rr.ServerName = name
	rr.ServerVersion = version
}
func (rr *RequestInfo) SetHostname(hostname string) {
	rr.ServerHostname = hostname
}
func (rr *RequestInfo) ReadURL(u *url.URL) {
	rr.Path = u.String()
	rr.URL = u
}
func (rr *RequestInfo) ReadForm(r *http.Request) {
	r.ParseForm()
	rr.Form = r.Form
}
func (rr *RequestInfo) ReadTLS(r *http.Request) {
	if r.TLS != nil {
		rr.TLS.Protocol = r.TLS.NegotiatedProtocol
		switch r.TLS.Version {
		default:
			rr.TLS.Version = tls.VersionName(r.TLS.Version)
		}
		rr.TLS.Cipher = strconv.Itoa(int(r.TLS.CipherSuite))
		rr.TLS.CipherName = tls.CipherSuiteName(r.TLS.CipherSuite)
	}
}
func (rr *RequestInfo) ReadBody(r io.ReadCloser) error {
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(b) <= 1024 {
		rr.Body = b
	} else {
		rr.Body = bytes.Join([][]byte{b[:1024], []byte("...")}, []byte{})
	}
	return nil
}
func (rr *RequestInfo) DecodeAuthorization(authorization string) {
	if authorization != "" {
		if strings.HasPrefix(authorization, "Basic ") {
			if auth, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authorization, "Basic ")); err == nil {
				a := strings.SplitN(string(auth), ":", 2)
				rr.BasicAuth = &(struct {
					Username string `json:"username,omitempty"`
					Password string `json:"password,omitempty"`
				}{Username: a[0], Password: a[1]})
			}
		} else if strings.HasPrefix(authorization, "Bearer ") {
			tab := strings.Split(strings.TrimPrefix(authorization, "Bearer "), ".")
			if len(tab) == 3 {
				for i := 0; i < len(tab); i++ {
					for len(tab[i])%4 != 0 {
						tab[i] += "="
					}
					if a, err := base64.StdEncoding.DecodeString(tab[i]); err == nil {
						tab[i] = string(a)
					}
				}
				jwt := `{"header":` + tab[0] + `,"payload":` + tab[1] + `,"signature":null}`
				if json.Unmarshal([]byte(jwt), &rr.JWT) == nil {
					if root, ok := rr.JWT.(map[string]interface{}); ok {
						if payload, ok := root["payload"].(map[string]interface{}); ok {
							if iat, ok := payload["iat"].(float64); ok {
								issuedAt := time.Unix(int64(iat), 0).Format(time.RFC3339)
								payload["issuedAt"] = issuedAt
							}
						}
					}
				}
			}
		}
	}
}
func (rr *RequestInfo) Read(r *http.Request) error {
	rr.Proto = r.Proto
	rr.ProtoMajor = r.ProtoMajor
	rr.ProtoMinor = r.ProtoMinor
	rr.Method = r.Method
	rr.Host = r.Host
	rr.ReadURL(r.URL)
	rr.Headers = r.Header
	rr.Trailers = r.Trailer
	rr.RemoteAddr = r.RemoteAddr
	rr.Close = r.Close
	rr.ContentLength = r.ContentLength
	rr.TransferEncoding = r.TransferEncoding
	rr.RequestURI = r.RequestURI
	rr.ReadForm(r)
	rr.ReadBody(r.Body)
	rr.ReadTLS(r)
	rr.DecodeAuthorization(r.Header.Get("Authorization"))
	if len(rr.ServerName) > 0 {
		rr.Server = rr.ServerName
	} else {
		rr.Server = "unknown"
	}
	if len(rr.ServerVersion) > 0 {
		rr.Server = rr.Server + "/" + rr.ServerVersion
	} else {
		rr.Server = rr.Server + "/undefined"
	}
	if len(rr.ServerHostname) > 0 {
		rr.Server = rr.Server + " on " + rr.ServerHostname
	}
	rr.ReadHostInfo()
	return nil
}
func (rr *RequestInfo) SetStatusCode(status int) {
	rr.StatusCode = status
}
func (rr *RequestInfo) WriteText(rw http.ResponseWriter) error {
	if len(rr.ServerHostname) > 0 {
		rw.Header().Set("X-Hostname", rr.ServerHostname)
	}
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	rw.Header().Set("Expires", "0")

	if (rr.StatusCode == 204) || (rr.StatusCode == 304) { // no body for these codes
		rw.WriteHeader(rr.StatusCode)
	} else if rr.Method == "HEAD" { // no body for this method
		rw.Header().Set("Content-Type", "plain/text")
		if rr.StatusCode > 0 {
			rw.WriteHeader(rr.StatusCode)
		} else {
			rw.WriteHeader(http.StatusOK)
		}
	} else {
		rw.Header().Set("Content-Type", "text/plain")
		if rr.StatusCode > 0 {
			rw.WriteHeader(rr.StatusCode)
		} else {
			if sc, err := strconv.Atoi(rr.URL.Path[1:]); err == nil {
				if txt := http.StatusText(sc); len(txt) > 0 {
					rw.WriteHeader(sc)
				} else {
					rw.WriteHeader(http.StatusOK)
				}
			} else {
				rw.WriteHeader(http.StatusOK)
			}
		}
		rw.Write([]byte(rr.Method + " " + rr.Path + "\n"))
		if len(rr.Headers) > 0 {
			for n, v := range rr.Headers {
				rw.Write([]byte(n + ": " + strings.Join(v, ",") + "\n"))
			}
		}
		rw.Write([]byte("\n")) // Write the blank line separating the headers from the body.
		rw.Write(rr.Body)
	}
	return nil
}
func (rr *RequestInfo) WriteJson(rw http.ResponseWriter) error {
	if len(rr.ServerHostname) > 0 {
		rw.Header().Set("X-Hostname", rr.ServerHostname)
	}
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	rw.Header().Set("Expires", "0")

	if (rr.StatusCode == 204) || (rr.StatusCode == 304) { // no body for these codes
		rw.WriteHeader(rr.StatusCode)
	} else if rr.Method == "HEAD" { // no body for this method
		rw.Header().Set("Content-Type", "application/json")
		if rr.StatusCode > 0 {
			rw.WriteHeader(rr.StatusCode)
		} else {
			rw.WriteHeader(http.StatusOK)
		}
	} else {
		rrb, err := json.Marshal(rr)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return err
		}
		rw.Header().Set("Content-Type", "application/json")
		if rr.StatusCode > 0 {
			rw.WriteHeader(rr.StatusCode)
		} else {
			if sc, err := strconv.Atoi(rr.URL.Path[1:]); err == nil {
				if txt := http.StatusText(sc); len(txt) > 0 {
					rw.WriteHeader(sc)
				} else {
					rw.WriteHeader(http.StatusOK)
				}
			} else {
				rw.WriteHeader(http.StatusOK)
			}
		}
		rw.Write(rrb)
	}
	return nil
}

/***************************/

func echoHandler(rw http.ResponseWriter, r *http.Request) {
	// Preparing request info structure
	var err error
	rr := &RequestInfo{}
	// Set server name
	rr.SetServer("gorp", "0.9")
	if h, err := os.Hostname(); err == nil {
		rr.SetHostname(h)
	}
	err = rr.Read(r)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	status := 200
	rr.SetStatusCode(status)
	if status < 0 {
		rr.WriteText(rw)
	} else {
		rr.WriteJson(rw)
	}
}
