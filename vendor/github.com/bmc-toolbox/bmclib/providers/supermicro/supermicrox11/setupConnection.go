package supermicrox11

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/bmc-toolbox/bmclib/errors"
	"github.com/bmc-toolbox/bmclib/internal/httpclient"
	"github.com/bmc-toolbox/bmclib/providers/supermicro"
	log "github.com/sirupsen/logrus"
)

// httpLogin initiates the connection to an SupermicroX device
func (s *SupermicroX) httpLogin() (err error) {
	if s.httpClient != nil {
		return
	}

	httpClient, err := httpclient.Build()
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{"step": "bmc connection", "vendor": supermicro.VendorID, "ip": s.ip}).Debug("connecting to bmc")

	data := fmt.Sprintf("name=%s&pwd=%s", s.username, s.password)
	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/cgi/login.cgi", s.ip), bytes.NewBufferString(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode == 404 {
		return errors.ErrPageNotFound
	}

	payload, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !strings.Contains(string(payload), "../cgi/url_redirect.cgi?url_name=mainmenu") {
		return errors.ErrLoginFailed
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" && cookie.Value != "" {
			s.sid = cookie
		} else {
			s.sid = &http.Cookie{}
		}
	}

	s.httpClient = httpClient

	return err
}

// Close closes the connection properly
func (s *SupermicroX) Close(ctx context.Context) (err error) {
	if s.httpClient != nil {
		bmcURL := fmt.Sprintf("https://%s/cgi/logout.cgi", s.ip)
		log.WithFields(log.Fields{"step": "bmc connection", "vendor": supermicro.VendorID, "ip": s.ip}).Debug("logout from bmc")

		req, err := http.NewRequest("POST", bmcURL, nil)
		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		u, err := url.Parse(bmcURL)
		if err != nil {
			return err
		}
		for _, cookie := range s.httpClient.Jar.Cookies(u) {
			if cookie.Name == "SID" && cookie.Value != "" {
				req.AddCookie(cookie)
			}
		}
		if log.GetLevel() == log.TraceLevel {
			log.Trace(fmt.Sprintf("%s/cgi/%s", bmcURL, s.ip))
			dump, err := httputil.DumpRequestOut(req, true)
			if err == nil {
				log.WithFields(log.Fields{
					"type": "Request",
					"when": "Before HTTP Action",
					"dump": string(dump),
				}).Trace("")
			}
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return err
		}

		defer resp.Body.Close()
		defer io.Copy(ioutil.Discard, resp.Body) // nolint

		if log.GetLevel() == log.TraceLevel {
			log.Trace(fmt.Sprintf("%s/cgi/%s", bmcURL, s.ip))
			dump, err := httputil.DumpRequestOut(req, true)
			if err == nil {
				log.WithFields(log.Fields{
					"type": "Request",
					"when": "After HTTP Action",
					"dump": string(dump),
				}).Trace("")
			}
		}

		return err
	}
	return err
}
