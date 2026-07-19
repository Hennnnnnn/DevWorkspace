package actions

import (
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// ListDevices returns the caller's registered devices.
func ListDevices() ([]protocol.Device, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var out protocol.DeviceList
	if err := cl.Get("/devices", nil, &out); err != nil {
		return nil, err
	}
	return out.Devices, nil
}

// RevokeDevice revokes one of the caller's own devices by ID.
func RevokeDevice(deviceID string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	return cl.Post("/devices/link", protocol.RevokeRequest{DeviceID: deviceID}, nil)
}
