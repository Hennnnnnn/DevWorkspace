package actions

import (
	"fmt"
	"os"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/api"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// RegisterResult is the outcome of registering a device.
type RegisterResult struct {
	Username    string
	Status      string // pending | active
	Fingerprint string
}

// Register registers this device's public key with the server. deviceName
// defaults to the hostname when empty.
func Register(username, deviceName, passphrase string) (*RegisterResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url not set — run `devsync config set server_url <url>`")
	}
	if deviceName == "" {
		host, _ := os.Hostname()
		deviceName = host
	}
	kp, err := keystore.Unlock(passphrase)
	if err != nil {
		return nil, err
	}

	req := protocol.RegisterRequest{
		Username:    username,
		DeviceName:  deviceName,
		SignPubKey:  kp.SignPub,
		BoxPubKey:   kp.BoxPub[:],
		Fingerprint: crypto.Fingerprint(kp.SignPub),
	}
	var resp protocol.RegisterResponse
	if err := api.PostUnsigned(cfg.ServerURL, "/register", req, &resp); err != nil {
		return nil, err
	}
	// Persist identity for future signed requests.
	cfg.Username = username
	cfg.DeviceID = resp.DeviceID
	if err := cfg.Save(); err != nil {
		return nil, err
	}
	return &RegisterResult{Username: username, Status: resp.Status, Fingerprint: req.Fingerprint}, nil
}

// WhoAmI returns the current identity and device status.
func WhoAmI() (*protocol.WhoAmIResponse, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var resp protocol.WhoAmIResponse
	if err := cl.Get("/whoami", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
