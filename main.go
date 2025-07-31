package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	"gopkg.in/yaml.v3"

	_ "github.com/xjasonlyu/tun2socks/v2/dns"
	"github.com/xjasonlyu/tun2socks/v2/engine"
	"github.com/xjasonlyu/tun2socks/v2/internal/version"
	"github.com/xjasonlyu/tun2socks/v2/log"
)

var (
	key = new(engine.Key)

	configFile          string
	versionFlag         bool
	proxyFlag           string
	healthCheckEnable   bool
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	healthCheckURL      string
)

func init() {
	flag.IntVar(&key.Mark, "fwmark", 0, "Set firewall MARK (Linux only)")
	flag.IntVar(&key.MTU, "mtu", 0, "Set device maximum transmission unit (MTU)")
	flag.DurationVar(&key.UDPTimeout, "udp-timeout", 0, "Set timeout for each UDP session")
	flag.StringVar(&configFile, "config", "", "YAML format configuration file")
	flag.StringVar(&key.Device, "device", "", "Use this device [driver://]name")
	flag.StringVar(&key.Interface, "interface", "", "Use network INTERFACE (Linux/MacOS only)")
	flag.StringVar(&key.LogLevel, "loglevel", "info", "Log level [debug|info|warn|error|silent]")
	flag.StringVar(&proxyFlag, "proxy", "", "Use this proxy [protocol://]host[:port]")
	flag.StringVar(&key.RestAPI, "restapi", "", "HTTP statistic server listen address")
	flag.StringVar(&key.TCPSendBufferSize, "tcp-sndbuf", "", "Set TCP send buffer size for netstack")
	flag.StringVar(&key.TCPReceiveBufferSize, "tcp-rcvbuf", "", "Set TCP receive buffer size for netstack")
	flag.BoolVar(&key.TCPModerateReceiveBuffer, "tcp-auto-tuning", false, "Enable TCP receive buffer auto-tuning")
	flag.StringVar(&key.MulticastGroups, "multicast-groups", "", "Set multicast groups, separated by commas")
	flag.StringVar(&key.TUNPreUp, "tun-pre-up", "", "Execute a command before TUN device setup")
	flag.StringVar(&key.TUNPostUp, "tun-post-up", "", "Execute a command after TUN device setup")
	flag.BoolVar(&healthCheckEnable, "health-check", false, "Enable proxy health check")
	flag.DurationVar(&healthCheckInterval, "health-check-interval", 0, "Health check interval (default: 30s)")
	flag.DurationVar(&healthCheckTimeout, "health-check-timeout", 0, "Health check timeout (default: 5s)")
	flag.StringVar(&healthCheckURL, "health-check-url", "", "Health check target URL (default: http://www.google.com)")
	flag.BoolVar(&versionFlag, "version", false, "Show version and then quit")
	flag.Parse()
}

func main() {
	maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))

	if versionFlag {
		fmt.Println(version.String())
		fmt.Println(version.BuildString())
		os.Exit(0)
	}

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			log.Fatalf("Failed to read config file '%s': %v", configFile, err)
		}
		if err = yaml.Unmarshal(data, key); err != nil {
			log.Fatalf("Failed to unmarshal config file '%s': %v", configFile, err)
		}
	}

	// Handle command line proxy flag
	if proxyFlag != "" {
		if err := key.Proxy.UnmarshalYAML(&yaml.Node{Kind: yaml.ScalarNode, Value: proxyFlag}); err != nil {
			log.Fatalf("Failed to parse proxy flag: %v", err)
		}
	}

	// Handle command line health check flags
	if healthCheckEnable {
		key.HealthCheck.Enable = healthCheckEnable
	}
	if healthCheckInterval > 0 {
		key.HealthCheck.Interval = healthCheckInterval
	}
	if healthCheckTimeout > 0 {
		key.HealthCheck.Timeout = healthCheckTimeout
	}
	if healthCheckURL != "" {
		key.HealthCheck.URL = healthCheckURL
	}

	engine.Insert(key)

	engine.Start()
	defer engine.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
