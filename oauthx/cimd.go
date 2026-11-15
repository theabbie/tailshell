package oauthx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"syscall"
	"time"
)

// A client id that is an https URL names a Client ID Metadata Document that the
// client hosts itself. Fetching it is a server-side request to a URL the caller
// chose, so it only reaches public addresses and follows no redirects.
var cimdClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				ip := net.ParseIP(host)
				if err != nil || ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
					return errors.New("client metadata host is not a public address")
				}
				return nil
			},
		}).DialContext,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func isCIMD(id string) bool { return strings.HasPrefix(id, "https://") }

func fetchCIMD(ctx context.Context, id string) (client, error) {
	u, err := url.Parse(id)
	if err != nil || len(id) > 512 || u.Host == "" || u.Path == "" || u.Path == "/" || u.Fragment != "" || u.User != nil {
		return client{}, errors.New("client_id is not a valid metadata document URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return client{}, errors.New("client_id is not a valid metadata document URL")
	}
	req.Header.Set("accept", "application/json")
	resp, err := cimdClient.Do(req)
	if err != nil {
		return client{}, errors.New("could not fetch the client metadata document")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return client{}, errors.New("could not fetch the client metadata document")
	}
	var doc struct {
		ClientID     string   `json:"client_id"`
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
		AuthMethod   string   `json:"token_endpoint_auth_method"`
		AuthMethods  []string `json:"token_endpoint_auth_methods_supported"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&doc) != nil || doc.ClientID != id {
		return client{}, errors.New("client metadata document is invalid or its client_id does not match its URL")
	}
	if doc.AuthMethod != "" && doc.AuthMethod != "none" && !slices.Contains(doc.AuthMethods, "none") {
		return client{}, errors.New("only public clients (token_endpoint_auth_method none) are supported")
	}
	if len(doc.RedirectURIs) == 0 || len(doc.RedirectURIs) > 5 {
		return client{}, errors.New("client metadata document needs 1 to 5 redirect_uris")
	}
	for _, r := range doc.RedirectURIs {
		if !validRedirect(r) {
			return client{}, errors.New("client metadata redirect URIs must use https (or http on localhost) without a fragment")
		}
	}
	name := strings.TrimSpace(doc.ClientName)
	if len(name) > 80 {
		name = name[:80]
	}
	if name == "" {
		name = u.Host
	}
	return client{Name: name, Redirects: doc.RedirectURIs}, nil
}
