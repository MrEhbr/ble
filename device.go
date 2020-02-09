package ble

import (
	"context"
	"fmt"
	"log"

	"github.com/godbus/dbus"
)

const (
	deviceInterface = "org.bluez.Device1"
	interfacesAdded = "org.freedesktop.DBus.ObjectManager.InterfacesAdded"
)

// The Device type corresponds to the org.bluez.Device1 interface.
// See bluez/doc/devicet-api.txt
type Device interface {
	BaseObject

	UUIDs() []string
	Connected() bool
	Paired() bool

	Connect() error
	Disconnect() error
	Pair() error

	WatchProperties(context.Context, func(props Properties)) error
	ServiceData() map[string]dbus.Variant
}

func (conn *Connection) matchDevice(matching predicate) (Device, error) {
	return conn.findObject(deviceInterface, matching)
}

// GetDevice finds a Device in the object cache matching the given UUIDs.
func (conn *Connection) GetDevice(uuids ...string) (Device, error) {
	return conn.matchDevice(func(device *blob) bool {
		return uuidsInclude(device.UUIDs(), uuids)
	})
}

func uuidsInclude(advertised []string, uuids []string) bool {
	for _, u := range uuids {
		if !ValidUUID(u) {
			log.Printf("invalid UUID %s", u)
			return false
		}
		if !stringArrayContains(advertised, u) {
			return false
		}
	}
	return true
}

// GetDeviceByName finds a Device in the object cache with the given name.
func (conn *Connection) GetDeviceByName(name string) (Device, error) {
	return conn.matchDevice(func(device *blob) bool {
		return device.Name() == name
	})
}

// GetDeviceByAddress finds a Device in the object cache with the given name.
func (conn *Connection) GetDeviceByAddress(addr string) (Device, error) {
	return conn.matchDevice(func(device *blob) bool {
		return device.Address() == addr
	})
}

func (device *blob) UUIDs() []string {
	return device.properties["UUIDs"].Value().([]string)
}

func (device *blob) Connected() bool {
	return device.properties["Connected"].Value().(bool)
}

func (device *blob) Paired() bool {
	return device.properties["Paired"].Value().(bool)
}

func (device *blob) Address() string {
	return device.properties["Address"].Value().(string)
}

func (device *blob) ServiceData() map[string]dbus.Variant {
	return device.properties["ServiceData"].Value().(map[string]dbus.Variant)
}

func (device *blob) Connect() error {
	log.Printf("%s: connecting", device.Name())
	return device.call("Connect")
}

func (device *blob) Disconnect() error {
	log.Printf("%s: disconnecting", device.Name())
	return device.call("Disconnect")
}

func (device *blob) WatchProperties(ctx context.Context, h func(props Properties)) error {
	rule := fmt.Sprintf(
		"type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='%s'",
		device.Path(),
	)

	err := device.conn.addMatch(rule)
	if err != nil {
		return err
	}

	c := make(chan *dbus.Signal, 10)
	device.conn.bus.Signal(c)

	defer func() {
		_ = device.conn.removeMatch(rule)
		device.conn.bus.RemoveSignal(c)
		close(c)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-c:
			// Reflection used by dbus.Store() requires explicit type here.
			var changed map[string]dbus.Variant
			_ = dbus.Store(sig.Body[1:2], &changed)
			h(changed)
		}
	}

	return nil
}

func (device *blob) Pair() error {
	log.Printf("%s: pairing", device.Name())
	return device.call("Pair")
}

func stringArrayContains(a []string, str string) bool {
	for _, s := range a {
		if s == str {
			return true
		}
	}
	return false
}
