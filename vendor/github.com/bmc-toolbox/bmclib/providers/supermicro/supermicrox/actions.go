package supermicrox

import (
	"fmt"

	"github.com/bmc-toolbox/bmclib/internal/ipmi"
)

// PowerCycle reboots the machine via bmc
func (s *SupermicroX) PowerCycle() (status bool, err error) {
	i, err := ipmi.New(s.username, s.password, s.ip)
	if err != nil {
		return status, err
	}
	status, err = i.PowerCycle()
	return status, err
}

// PowerCycleBmc reboots the bmc we are connected to
func (s *SupermicroX) PowerCycleBmc() (status bool, err error) {
	i, err := ipmi.New(s.username, s.password, s.ip)
	if err != nil {
		return status, err
	}
	status, err = i.PowerCycleBmc()
	return status, err
}

// PowerOn power on the machine via bmc
func (s *SupermicroX) PowerOn() (status bool, err error) {
	i, err := ipmi.New(s.username, s.password, s.ip)
	if err != nil {
		return status, err
	}
	status, err = i.PowerOn()
	return status, err
}

// PowerOff power off the machine via bmc
func (s *SupermicroX) PowerOff() (status bool, err error) {
	i, err := ipmi.New(s.username, s.password, s.ip)
	if err != nil {
		return status, err
	}
	status, err = i.PowerOff()
	return status, err
}

// PxeOnce makes the machine to boot via pxe once
func (s *SupermicroX) PxeOnce() (status bool, err error) {
	i, err := ipmi.New(s.username, s.password, s.ip)
	if err != nil {
		return status, err
	}
	_, err = i.PxeOnceEfi()
	if err != nil {
		return false, err
	}
	return i.PowerCycle()
}

// IsOn tells if a machine is currently powered on
func (s *SupermicroX) IsOn() (status bool, err error) {
	i, err := ipmi.New(s.username, s.password, s.ip)
	if err != nil {
		return status, err
	}
	status, err = i.IsOn()
	return status, err
}

// UpdateFirmware updates the bmc firmware
func (s *SupermicroX) UpdateFirmware(source, file string) (status bool, err error) {
	return true, fmt.Errorf("not supported yet")
}
