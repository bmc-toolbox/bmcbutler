// Package registrar implements a variant of the registration pattern.
package registrar

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
)

// Feature represents a single feature a driver supports.
type Feature string

// Features holds the features a driver supports.
type Features []Feature

// Drivers holds a slice of Driver types.
type Drivers []*Driver

// Option for setting optional Registry values.
type Option func(*Registry)

// Verifier allows implementations to define a method for
// determining whether a driver is compatible for use.
type Verifier interface {
	Compatible(context.Context) bool
}

// Registry holds the registered drivers.
type Registry struct {
	Logger  logr.Logger
	Drivers Drivers
}

// Driver holds the info about a driver.
type Driver struct {
	Name            string
	Protocol        string
	Features        Features
	Metadata        interface{}
	DriverInterface interface{}
}

// WithLogger sets the logger.
func WithLogger(logger logr.Logger) Option {
	return func(args *Registry) { args.Logger = logger }
}

// WithDrivers sets the drivers.
func WithDrivers(drivers Drivers) Option {
	return func(args *Registry) { args.Drivers = drivers }
}

// NewRegistry returns a new Driver registry.
func NewRegistry(opts ...Option) *Registry {
	defaultRegistry := &Registry{
		Logger: logr.Discard(),
	}
	for _, opt := range opts {
		opt(defaultRegistry)
	}

	return defaultRegistry
}

// Register will add a driver a Driver registry.
func (r *Registry) Register(name, protocol string, features Features, metadata interface{}, driverInterface interface{}) {
	r.Drivers = append(r.Drivers, &Driver{
		Name:            name,
		Protocol:        protocol,
		Features:        features,
		Metadata:        metadata,
		DriverInterface: driverInterface,
	})
}

// GetDriverInterfaces returns a slice of just the generic driver interfaces.
func (r Registry) GetDriverInterfaces() []interface{} {
	var results []interface{}
	for _, elem := range r.Drivers {
		if elem != nil {
			results = append(results, elem.DriverInterface)
		}
	}
	return results
}

// FilterForCompatible updates the driver registry with only compatible implementations.
// compatible implementations are determined by running the Compatible method of the Verifier
// interface. registered drivers must implement the Verifier interface for this.
func (r Registry) FilterForCompatible(ctx context.Context) Drivers {
	var wg sync.WaitGroup
	mutex := &sync.Mutex{}
	state := make(map[int]*Driver)

	for index, elem := range r.Drivers {
		index := index
		wg.Add(1)
		go func(isCompat interface{}, reg *Driver, wg *sync.WaitGroup) {
			switch c := isCompat.(type) {
			case Verifier:
				if c.Compatible(ctx) {
					mutex.Lock()
					state[index] = reg
					mutex.Unlock()
				}
			default:
				mutex.Lock()
				state[index] = reg
				mutex.Unlock()
				r.Logger.V(1).Info(fmt.Sprintf("could not check for compatibility. not a Verifier implementation: %T", c))
			}
			wg.Done()
		}(elem.DriverInterface, elem, &wg)
	}
	wg.Wait()

	var result Drivers
	for i := 0; i < len(state); i++ {
		if state[i] != nil {
			result = append(result, state[i])
		}
	}

	return result
}

// include does the actual work of filtering for specific features.
func (f Features) include(features ...Feature) bool {
	if len(features) > len(f) {
		return false
	}
	fKeys := make(map[Feature]bool)
	for _, v := range f {
		fKeys[v] = true
	}
	for _, f := range features {
		if _, ok := fKeys[f]; !ok {
			return false
		}
	}
	return true
}

// Supports does the actual work of filtering for specific features.
func (r Registry) Supports(features ...Feature) Drivers {
	var supportedRegistries Drivers
	for _, reg := range r.Drivers {
		if reg.Features.include(features...) {
			supportedRegistries = append(supportedRegistries, reg)
		}
	}
	return supportedRegistries
}

// Using does the actual work of filtering for a specific protocol type.
func (r Registry) Using(proto string) Drivers {
	var supportedRegistries Drivers
	for _, reg := range r.Drivers {
		if reg.Protocol == proto {
			supportedRegistries = append(supportedRegistries, reg)
		}
	}
	return supportedRegistries
}

// For does the actual work of filtering for a specific driver name.
func (r Registry) For(driver string) Drivers {
	var supportedRegistries Drivers
	for _, reg := range r.Drivers {
		if reg.Name == driver {
			supportedRegistries = append(supportedRegistries, reg)
		}
	}
	return supportedRegistries
}

// deduplicate returns a new slice with duplicates values removed.
func deduplicate(s []string) []string {
	if len(s) <= 1 {
		return s
	}
	result := []string{}
	seen := make(map[string]struct{})
	for _, val := range s {
		val := strings.ToLower(val)
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = struct{}{}
		}
	}
	return result
}

// PreferProtocol does the actual work of moving preferred protocols to the start of the driver registry.
func (r Registry) PreferProtocol(protocols ...string) Drivers {
	var final Drivers
	var leftOver Drivers
	tracking := make(map[int]Drivers)
	protocols = deduplicate(protocols)
	for _, registry := range r.Drivers {
		var movedToTracking bool
		for index, pName := range protocols {
			if strings.EqualFold(registry.Protocol, pName) {
				tracking[index] = append(tracking[index], registry)
				movedToTracking = true
			}
		}
		if !movedToTracking {
			leftOver = append(leftOver, registry)
		}
	}
	for x := 0; x <= len(tracking); x++ {
		final = append(final, tracking[x]...)
	}
	final = append(final, leftOver...)
	return final
}

// PreferDriver will reorder the registry by moving preferred drivers to the start.
func (r Registry) PreferDriver(drivers ...string) Drivers {
	var final Drivers
	var leftOver Drivers
	tracking := make(map[int]Drivers)
	drivers = deduplicate(drivers)
	for _, registry := range r.Drivers {
		var movedToTracking bool
		for index, pName := range drivers {
			if strings.EqualFold(registry.Name, pName) {
				tracking[index] = append(tracking[index], registry)
				movedToTracking = true
			}
		}
		if !movedToTracking {
			leftOver = append(leftOver, registry)
		}
	}
	for x := 0; x <= len(tracking); x++ {
		final = append(final, tracking[x]...)
	}
	final = append(final, leftOver...)
	return final
}
