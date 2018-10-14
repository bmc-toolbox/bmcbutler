### bmcbutler - A BMC configuration tool

Bmcbutler is a tool to configure BMCs using [bmclib](https://github.com/ncode/bmclib),
assets - BMCs to configure are read from an inventory source defined in `bmcbutler.yml` configuration file.

Multiple butler processes are spawned to configure BMCs based on configuration declared in [configuration.yml sample](../master/cfg/configuration.yml)

For supported BMCs see the bmclib page.

##### Build
`go get github.com/bmc-toolbox/bmcbutler`

##### Setup
Theres two parts to setting up configuration for bmcbutler,

* Bmcbutler configuration
* Configuration for BMCs

This document assumes the Bmcbutler configuration directory is ~/.bmcbutler.

###### Bmcbutler configuration
Setup configuration Bmcbutler requires to run.

```
# create a configuration directory for ~/.bmcbutler
mkdir ~/.bmcbutler/
```
Copy the sample config into ~/.bmcbutler/
[bmcbutler.yml sample](../master/samples/bmcbutler.yml.sample)

###### BMC configuration
Configuration to be applied to BMCs.

BMC configuration is split into two types,

* configuration - configuration to be applied periodically.

```
# create a directory for BMC config
mkdir ~/.bmcbutler/cfg
```
add the BMC yaml config definitions in there, for sample config see [configuration.yml sample](../master/cfg/configuration.yml)

###### bmc configuration templating
configuration.yml supports templating, for details see [configTemplating](../master/docs/configTemplating.md)

###### inventory
Bmcbutler was written with the intent of sourcing inventory assets and configuring their bmcs,
a csv inventory example is provided to play with.

[inventory.csv sample](../master/samples/inventory.csv.sample)

The 'inventory' parameter points Bmcbutler to the inventory source.


##### Run

Configure Blades/Chassis/Discretes

```
#configure all BMCs in inventory, dry run with verbose output
bmcbutler configure --all --dryrun -v

#configure all blades in given locations
bmcbutler configure --blades --locations ams2

#configure all chassis in given locations
bmcbutler configure --chassis --locations ams2,lhr3 

#configure all discretes in given location, spawning given butlers
bmcbutler configure --discretes --locations lhr5 --butlers 200

#configure one or more BMCs identified by IP(s)
bmcbutler configure --ips 192.168.0.1,192.168.0.2,192.168.0.2

#configure one or more BMCs identified by serial(s)
bmcbutler configure --serials <serial1>,<serial2>

bmcbutler configure --blade --serial <serial1>,<serial2> --verbose
bmcbutler configure --discrete --serial <serial> --verbose
```

#### Acknowledgment

bmcbutler was originally developed for [Booking.com](http://www.booking.com).
With approval from [Booking.com](http://www.booking.com), the code and
specification were generalized and published as Open Source on github, for
which the authors would like to express their gratitude.
