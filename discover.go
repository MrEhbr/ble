package ble

import (
	"fmt"
	"log"
	"time"

	"github.com/godbus/dbus"
)

func addMatch(rule string) error {
	return bus.BusObject().Call(
		"org.freedesktop.DBus.AddMatch",
		0,
		rule,
	).Err
}

func removeMatch(rule string) error {
	return bus.BusObject().Call(
		"org.freedesktop.DBus.RemoveMatch",
		0,
		rule,
	).Err
}

func (adapter *blob) Discover(timeout time.Duration, uuids ...string) error {
	signals := make(chan *dbus.Signal)
	defer close(signals)
	bus.Signal(signals)
	defer bus.RemoveSignal(signals)
	rule := "type='signal',interface='org.freedesktop.DBus.ObjectManager',member='InterfacesAdded'"
	err := addMatch(rule)
	if err != nil {
		return err
	}
	defer removeMatch(rule)
	err = adapter.SetDiscoveryFilter(uuids...)
	if err != nil {
		return err
	}
	err = adapter.StartDiscovery()
	if err != nil {
		return err
	}
	defer adapter.StopDiscovery()
	var t <-chan time.Time
	if timeout != 0 {
		t = time.After(timeout)
	}
	for {
		select {
		case s := <-signals:
			switch s.Name {
			case interfacesAdded:
				if containsDevice(s) {
					log.Printf("%s: discovery finished\n", adapter.Name())
					return nil
				}
				log.Printf("%s: skipping signal %s with no device interface\n", adapter.Name(), s.Name)
			default:
				log.Printf("%s: unexpected signal %s\n", adapter.Name(), s.Name)
			}
		case <-t:
			return fmt.Errorf("discovery timeout")
		}
	}
	return nil
}

// Check whether the InterfacesAdded signal contains deviceInterface
// See http://dbus.freedesktop.org/doc/dbus-specification.html#standard-interfaces-objectmanager
func containsDevice(s *dbus.Signal) bool {
	var dict map[string]map[string]dbus.Variant
	err := dbus.Store(s.Body[1:2], &dict)
	if err != nil {
		return false
	}
	return dict[deviceInterface] != nil
}

func (cache *ObjectCache) Discover(timeout time.Duration, uuids ...string) (Device, error) {
	device, err := cache.GetDevice(uuids...)
	if err == nil {
		log.Printf("%s: already discovered\n", device.Name())
		return device, nil
	}
	adapter, err := cache.GetAdapter()
	if err != nil {
		return nil, err
	}
	err = adapter.Discover(timeout, uuids...)
	if err != nil {
		return nil, err
	}
	err = cache.Update()
	if err != nil {
		log.Fatal(err)
	}
	return cache.GetDevice(uuids...)
}
