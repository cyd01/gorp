package helper

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func ReadFile(filename string) ([]byte, error) {
	var data []byte
	var err error
	if filename == "" {
		return data, fmt.Errorf("no file provided")
	}
	if strings.HasPrefix(filename, "http://") || strings.HasPrefix(filename, "https://") { // Read from an HTTP(S) URL.
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}
		if strings.HasPrefix(filename, "https://") {
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
		}
		var res *http.Response
		if res, err = client.Get(filename); err == nil { // Read from a web-server URL.
			defer res.Body.Close()
			if res.StatusCode != 204 { // if status code is 204, there is no content to read
				data, err = io.ReadAll(res.Body)
			}
			if err == nil && res.StatusCode != 200 && res.StatusCode != 204 {
				err = errors.New(http.StatusText(res.StatusCode))
				return []byte(""), err
			}
			return data, nil
		} else {
			return []byte(""), err
		}
	}
	if after, ok := strings.CutPrefix(filename, "file://"); ok { // Read from a file URL.
		filename = after
	}
	if data, err = os.ReadFile(filename); err != nil {
		var txt string
		var b bool
		if txt, b = os.LookupEnv(filename); b { // Read directly from an environment variable.
			data = []byte(txt)
		} else if txt, b = os.LookupEnv(strings.ToUpper(strings.ReplaceAll(filename, "-", "_"))); b { // Read from an uppercase environment variable.
			data = []byte(txt)
		} else { // Use the parameter itself as the content.
			data = []byte(filename)
			if len(data) == 0 {
				err = errors.New("can not get datas")
			}
		}
	}
	return data, nil
}
