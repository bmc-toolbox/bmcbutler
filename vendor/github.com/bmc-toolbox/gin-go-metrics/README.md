# gin-go-metrics

gin-go-metrics is [gin-gonic/gin](https://github.com/gin-gonic/gin) middleware to gather and store metrics using [rcrowley/go-metrics](https://github.com/rcrowley/go-metrics)

## How to use

### gin middleware

```go
package main

import (
	"fmt"
	"os"
	"time"

	metrics "github.com/bmc-toolbox/gin-go-metrics"
	"github.com/bmc-toolbox/gin-go-metrics/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	// Optional part to send metrics to Graphite,
	// as alternative you can send metrics from
	// rcrowley/go-metrics.DefaultRegistry yourself
	err := metrics.Setup(
		"graphite",  // clientType
		"localhost", // graphite host
		2003,        // graphite port
		"server",    // metrics prefix
		time.Minute, // graphite flushInterval
	)
	if err != nil {
		fmt.Printf("Failed to set up monitoring: %s\n", err)
		os.Exit(1)
	}

	r := gin.New()

	// argument to NewMetrics tells which variables need to be
	// expanded in metrics, more on that by link:
	// https://banzaicloud.com/blog/monitoring-gin-with-prometheus/
	p := middleware.NewMetrics([]string{"expanded_parameter"})
	r.Use(p.HandlerFunc(
		[]string{"/ping", "/api/ping"}, // ignore given URLs from stats
		true,                           // replace "/" with "_" in URLs to prevent splitting Graphite namespace
	))

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, "Hello world!")
	})

	r.Run(":8000")
}
```

### standalone metrics sending

```go
package main

import (
	"fmt"
	"os"
	"time"

	metrics "github.com/bmc-toolbox/gin-go-metrics"
)

func main() {
	err := metrics.Setup(
		"graphite",  // clientType
		"localhost", // graphite host
		2003,        // graphite port
		"server",    // metrics prefix
		time.Minute, // graphite flushInterval
	)
	if err != nil {
		fmt.Printf("Failed to set up monitoring: %s\n", err)
		os.Exit(1)
	}
	// collect data using provided functions with provided arguments once a minute
	go metrics.Scheduler(time.Minute, metrics.GoRuntimeStats, []string{})
	go metrics.Scheduler(time.Minute, metrics.MeasureRuntime, []string{"uptime"}, time.Now())

	//<...>
	metrics.IncrCounter([]string{"happy_routine", "happy_runs_counter"}, 1)
	metrics.UpdateGauge([]string{"happy_routine", "happiness_level"}, 9000)
	metrics.UpdateHistogram([]string{"happy_routine", "happiness_hit"}, 35)
	metrics.UpdateTimer([]string{"happy_time"}, time.Minute)
}
```

## Provided metrics

Request processing time and count of requests stored in [go-metrics.Timer](https://github.com/rcrowley/go-metrics/blob/master/timer.go)

Request and response size stored in [go-metrics.Histogram](https://github.com/rcrowley/go-metrics/blob/master/histogram.go)

## Data storage

Currently only helper function for sending data to Graphite with [cyberdelia/go-metrics-graphite](https://github.com/cyberdelia/go-metrics-graphite)
 is present, however, you can send data using
 [go-metrics.DefaultRegistry](https://github.com/rcrowley/go-metrics/blob/cf894ca225d73a7d5dbb4b3a922f4ae3608bb618/registry.go#L323) anywhere you want.

## Acknowledgment

This library was originally developed for [Booking.com](http://www.booking.com).
With approval from [Booking.com](http://www.booking.com), the code and
specification was generalized and published as Open Source on GitHub, for
which the authors would like to express their gratitude.
