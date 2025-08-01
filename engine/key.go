package engine

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type Key struct {
	MTU                      int           `yaml:"mtu"`
	Mark                     int           `yaml:"fwmark"`
	Proxy                    ProxyConfig   `yaml:"proxy"`
	RestAPI                  string        `yaml:"restapi"`
	Device                   string        `yaml:"device"`
	LogLevel                 string        `yaml:"loglevel"`
	Interface                string        `yaml:"interface"`
	TCPModerateReceiveBuffer bool          `yaml:"tcp-moderate-receive-buffer"`
	TCPSendBufferSize        string        `yaml:"tcp-send-buffer-size"`
	TCPReceiveBufferSize     string        `yaml:"tcp-receive-buffer-size"`
	MulticastGroups          string        `yaml:"multicast-groups"`
	TUNPreUp                 string        `yaml:"tun-pre-up"`
	TUNPostUp                string        `yaml:"tun-post-up"`
	UDPTimeout               time.Duration `yaml:"udp-timeout"`
	// 健康检查配置
	HealthCheck HealthCheckConfig `yaml:"health-check"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enable   bool          `yaml:"enable"`   // 是否启用健康检查
	Interval time.Duration `yaml:"interval"` // 检查间隔，默认30秒
	Timeout  time.Duration `yaml:"timeout"`  // 检查超时时间，默认5秒
	URL      string        `yaml:"url"`      // 检查的目标URL，默认http://www.google.com
}

// ProxyConfig supports both single proxy string and multiple proxy slice
type ProxyConfig struct {
	proxies []string
}

// UnmarshalYAML implements custom YAML unmarshaling to support both single string and slice formats
func (p *ProxyConfig) UnmarshalYAML(value *yaml.Node) error {
	// Try to unmarshal as slice first
	var proxies []string
	if err := value.Decode(&proxies); err == nil {
		p.proxies = proxies
		return nil
	}

	// If that fails, try to unmarshal as single string
	var proxy string
	if err := value.Decode(&proxy); err == nil {
		p.proxies = []string{proxy}
		return nil
	}

	return fmt.Errorf("proxy must be either a string or an array of strings")
}

// GetProxies returns the list of proxy URLs
func (p *ProxyConfig) GetProxies() []string {
	return p.proxies
}

// IsEmpty returns true if no proxies are configured
func (p *ProxyConfig) IsEmpty() bool {
	return len(p.proxies) == 0
}
