[![Test](https://github.com/jacobweinstock/registrar/workflows/Test/badge.svg)](https://github.com/jacobweinstock/registrar/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/jacobweinstock/registrar/branch/main/graph/badge.svg)](https://codecov.io/gh/jacobweinstock/registrar)
[![Go Report Card](https://goreportcard.com/badge/github.com/bmc-toolbox/bmclib)](https://goreportcard.com/report/github.com/jacobweinstock/registrar)
[![GoDoc](https://godoc.org/github.com/bmc-toolbox/bmclib?status.svg)](https://pkg.go.dev/github.com/jacobweinstock/registrar)

# Registrar

A Go library implementing a variant of the registration pattern

## Description

The registration, driver, strategy or singleton pattern is the idea that implementations of the same functionality are built up into a registry for use and/or selection at runtime. This pattern is used in a few places in the standard library with the `net/http`, `database/sql`, and `flag` packages. In my experience, most notably with the registration of `database/sql` drivers.

This library is a variant of this pattern. Its use case is to build up a registry of implementations which can be filtered or ordered and returned as a generic slice of `interface{}`. These interfaces can then be passed along to functions that can type assert or switch case on them to run concrete interface methods. The registration of a driver in this package is explicit  (`Registrar.Register()`). All [magic](https://peter.bourgon.org/blog/2017/06/09/theory-of-modern-go.html) is purposefully avoided. See the [examples](examples/) directory for code snippets.

## Background

This library is produced as a general library but was built from its original use-case in [bmclib](https://github.com/bmc-toolbox/bmclib/blob/master/registrar/registrar.go). The purpose of [bmclib](https://github.com/bmc-toolbox/bmclib) is to interact with Baseboard Management Controllers (BMC). There are many different ways to interact with a single BMC, so a registry of the different implementations is built up and then when a request to perform a single action (power state for example) is tried with all the different implementations until one works.

## Usage

```go
// create a registry
reg := registrar.NewRegistry()

// registry drivers
one := &driverOne{name: "driverOne", protocol: "tcp", metadata: "this is driver one", features: registrar.Features{registrar.Feature("always double checking")}}
two := &driverTwo{name: "driverTwo", protocol: "udp", metadata: "this is driver two", features: registrar.Features{registrar.Feature("set and forget")}}
reg.Register(one.name, one.protocol, one.features, one.metadata, one)
reg.Register(two.name, two.protocol, two.features, two.metadata, two)

// do some filtering
ctx := context.Background()
reg.Drivers = reg.Using("tcp")
reg.Drivers = reg.FilterForCompatible(ctx)

// get the interfaces for use
interfaces := reg.GetDriverInterfaces()

```

## References  

- <https://dave.cheney.net/2017/06/11/go-without-package-scoped-variables>
- <https://eli.thegreenplace.net/2019/design-patterns-in-gos-databasesql-package/>
- <https://peter.bourgon.org/blog/2017/06/09/theory-of-modern-go.html>
