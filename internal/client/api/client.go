// Package api is the signed HTTP client the CLI uses to talk to the server.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// Client signs requests with the given device keypair + fingerprint.
type Client struct {
	baseURL     string
	username    string
	fingerprint string
	kp          *crypto.KeyPair
	http        *http.Client
}

// New builds a client from saved config + an unlocked keypair.
func New(cfg *config.Config, kp *crypto.KeyPair) (*Client, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url not set — run `devsync config set server_url <url>`")
	}
	return &Client{
		baseURL:     cfg.ServerURL,
		username:    cfg.Username,
		fingerprint: crypto.Fingerprint(kp.SignPub),
		kp:          kp,
		http:        &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// doSigned performs a signed request. body may be nil.
func (c *Client) doSigned(method, path string, query url.Values, body any, out any) error {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBytes = b
	}
	ts := time.Now().Unix()
	msg := protocol.SigningString(method, path, protocol.BodyHash(bodyBytes), ts)
	sig := crypto.Sign(c.kp.SignPriv, msg)

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(protocol.HeaderUser, c.username)
	req.Header.Set(protocol.HeaderDevice, c.fingerprint)
	req.Header.Set(protocol.HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(protocol.HeaderSignature, sig)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		var e protocol.ErrorResponse
		if json.Unmarshal(respBody, &e) == nil && e.Error != "" {
			return fmt.Errorf("server: %s (%d)", e.Error, resp.StatusCode)
		}
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// Post/Get helpers.
func (c *Client) Post(path string, body, out any) error {
	return c.doSigned(http.MethodPost, path, nil, body, out)
}
func (c *Client) Get(path string, query url.Values, out any) error {
	return c.doSigned(http.MethodGet, path, query, nil, out)
}

// PostUnsigned is used only for /register (device not yet trusted).
func PostUnsigned(baseURL, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var e protocol.ErrorResponse
		if json.Unmarshal(respBody, &e) == nil && e.Error != "" {
			return fmt.Errorf("server: %s (%d)", e.Error, resp.StatusCode)
		}
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}
