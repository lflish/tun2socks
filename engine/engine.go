package engine

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/go-units"
	"github.com/google/shlex"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/xjasonlyu/tun2socks/v2/core"
	"github.com/xjasonlyu/tun2socks/v2/core/device"
	"github.com/xjasonlyu/tun2socks/v2/core/option"
	"github.com/xjasonlyu/tun2socks/v2/dialer"
	"github.com/xjasonlyu/tun2socks/v2/log"
	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy"
	"github.com/xjasonlyu/tun2socks/v2/proxy/proto"
	"github.com/xjasonlyu/tun2socks/v2/restapi"
	"github.com/xjasonlyu/tun2socks/v2/tunnel"
)

var (
	_engineMu sync.Mutex

	// _defaultKey holds the default key for the engine.
	_defaultKey *Key

	// _defaultProxy holds the default proxy for the engine.
	_defaultProxy proxy.Proxy

	// _defaultDevice holds the default device for the engine.
	_defaultDevice device.Device

	// _defaultStack holds the default stack for the engine.
	_defaultStack *stack.Stack

	// _healthChecker holds the health checker instance.
	_healthChecker *HealthChecker
)

// Start starts the default engine up.
func Start() {
	if err := start(); err != nil {
		log.Fatalf("[ENGINE] failed to start: %v", err)
	}
}

// Stop shuts the default engine down.
func Stop() {
	if err := stop(); err != nil {
		log.Fatalf("[ENGINE] failed to stop: %v", err)
	}
}

// Insert loads *Key to the default engine.
func Insert(k *Key) {
	_engineMu.Lock()
	_defaultKey = k
	_engineMu.Unlock()
}

func start() error {
	_engineMu.Lock()
	defer _engineMu.Unlock()

	if _defaultKey == nil {
		return errors.New("empty key")
	}

	for _, f := range []func(*Key) error{
		general,
		restAPI,
		netstack,
	} {
		if err := f(_defaultKey); err != nil {
			return err
		}
	}
	return nil
}

func stop() (err error) {
	_engineMu.Lock()
	// 停止健康检查器
	if _healthChecker != nil {
		_healthChecker.Stop()
		_healthChecker = nil
	}
	if _defaultDevice != nil {
		_defaultDevice.Close()
	}
	if _defaultStack != nil {
		_defaultStack.Close()
		_defaultStack.Wait()
	}
	_engineMu.Unlock()
	return nil
}

func execCommand(cmd string) error {
	parts, err := shlex.Split(cmd)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	_, err = exec.Command(parts[0], parts[1:]...).Output()
	return err
}

func general(k *Key) error {
	level, err := log.ParseLevel(k.LogLevel)
	if err != nil {
		return err
	}
	log.SetLogger(log.Must(log.NewLeveled(level)))

	if k.Interface != "" {
		iface, err := net.InterfaceByName(k.Interface)
		if err != nil {
			return err
		}
		dialer.DefaultDialer.InterfaceName.Store(iface.Name)
		dialer.DefaultDialer.InterfaceIndex.Store(int32(iface.Index))
		log.Infof("[DIALER] bind to interface: %s", k.Interface)
	}

	if k.Mark != 0 {
		dialer.DefaultDialer.RoutingMark.Store(int32(k.Mark))
		log.Infof("[DIALER] set fwmark: %#x", k.Mark)
	}

	if k.UDPTimeout > 0 {
		if k.UDPTimeout < time.Second {
			return errors.New("invalid udp timeout value")
		}
		tunnel.T().SetUDPTimeout(k.UDPTimeout)
	}
	return nil
}

func restAPI(k *Key) error {
	if k.RestAPI != "" {
		u, err := parseRestAPI(k.RestAPI)
		if err != nil {
			return err
		}
		host, token := u.Host, u.User.String()

		restapi.SetStatsFunc(func() tcpip.Stats {
			_engineMu.Lock()
			defer _engineMu.Unlock()

			// default stack is not initialized.
			if _defaultStack == nil {
				return tcpip.Stats{}
			}
			return _defaultStack.Stats()
		})

		go func() {
			if err := restapi.Start(host, token); err != nil {
				log.Errorf("[RESTAPI] failed to start: %v", err)
			}
		}()
		log.Infof("[RESTAPI] serve at: %s", u)
	}
	return nil
}

func netstack(k *Key) (err error) {
	if k.Proxy.IsEmpty() {
		return errors.New("empty proxy")
	}
	if k.Device == "" {
		return errors.New("empty device")
	}

	if k.TUNPreUp != "" {
		log.Infof("[TUN] pre-execute command: `%s`", k.TUNPreUp)
		if preUpErr := execCommand(k.TUNPreUp); preUpErr != nil {
			log.Errorf("[TUN] failed to pre-execute: %s: %v", k.TUNPreUp, preUpErr)
		}
	}

	defer func() {
		if k.TUNPostUp == "" || err != nil {
			return
		}
		log.Infof("[TUN] post-execute command: `%s`", k.TUNPostUp)
		if postUpErr := execCommand(k.TUNPostUp); postUpErr != nil {
			log.Errorf("[TUN] failed to post-execute: %s: %v", k.TUNPostUp, postUpErr)
		}
	}()

	proxies := k.Proxy.GetProxies()
	if len(proxies) == 1 {
		// Single proxy mode
		if _defaultProxy, err = parseProxy(proxies[0]); err != nil {
			return
		}
		tunnel.T().SetDialer(_defaultProxy)
	} else {
		// Multiple proxy mode - use round-robin proxy
		var proxyList []proxy.Proxy
		for _, proxyStr := range proxies {
			p, parseErr := parseProxy(proxyStr)
			if parseErr != nil {
				return parseErr
			}
			proxyList = append(proxyList, p)
		}
		roundRobinProxy := NewRoundRobinProxy(proxyList)
		_defaultProxy = roundRobinProxy
		tunnel.T().SetDialer(roundRobinProxy)

		// 启动健康检查器（仅在多代理模式下）
		if k.HealthCheck.Enable {
			_healthChecker = NewHealthChecker(k.HealthCheck, proxyList, func(healthyProxies []proxy.Proxy) {
				if rrProxy, ok := _defaultProxy.(*RoundRobinProxy); ok {
					rrProxy.UpdateProxies(healthyProxies)
					log.Infof("[ENGINE] 更新健康代理列表，当前健康代理数量: %d", len(healthyProxies))
				}
			})
			_healthChecker.Start()
		}
	}

	if _defaultDevice, err = parseDevice(k.Device, uint32(k.MTU)); err != nil {
		return
	}

	var multicastGroups []netip.Addr
	if multicastGroups, err = parseMulticastGroups(k.MulticastGroups); err != nil {
		return err
	}

	var opts []option.Option
	if k.TCPModerateReceiveBuffer {
		opts = append(opts, option.WithTCPModerateReceiveBuffer(true))
	}

	if k.TCPSendBufferSize != "" {
		size, err := units.RAMInBytes(k.TCPSendBufferSize)
		if err != nil {
			return err
		}
		opts = append(opts, option.WithTCPSendBufferSize(int(size)))
	}

	if k.TCPReceiveBufferSize != "" {
		size, err := units.RAMInBytes(k.TCPReceiveBufferSize)
		if err != nil {
			return err
		}
		opts = append(opts, option.WithTCPReceiveBufferSize(int(size)))
	}

	if _defaultStack, err = core.CreateStack(&core.Config{
		LinkEndpoint:     _defaultDevice,
		TransportHandler: tunnel.T(),
		MulticastGroups:  multicastGroups,
		Options:          opts,
	}); err != nil {
		return
	}

	log.Infof(
		"[STACK] %s://%s <-> %s://%s",
		_defaultDevice.Type(), _defaultDevice.Name(),
		_defaultProxy.Proto(), _defaultProxy.Addr(),
	)
	return nil
}

// RoundRobinProxy implements round-robin load balancing across multiple proxies
type RoundRobinProxy struct {
	proxies []proxy.Proxy
	counter uint64
	mu      sync.RWMutex
}

// NewRoundRobinProxy creates a new round-robin proxy with the given proxy list
func NewRoundRobinProxy(proxies []proxy.Proxy) *RoundRobinProxy {
	return &RoundRobinProxy{
		proxies: proxies,
		counter: 0,
	}
}

// UpdateProxies updates the proxy list dynamically
func (rr *RoundRobinProxy) UpdateProxies(proxies []proxy.Proxy) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.proxies = make([]proxy.Proxy, len(proxies))
	copy(rr.proxies, proxies)
}

// nextProxy returns the next proxy in round-robin fashion
func (rr *RoundRobinProxy) nextProxy() proxy.Proxy {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if len(rr.proxies) == 0 {
		return nil
	}

	n := atomic.AddUint64(&rr.counter, 1)
	return rr.proxies[(n-1)%uint64(len(rr.proxies))]
}

// DialContext implements proxy.Dialer interface
func (rr *RoundRobinProxy) DialContext(ctx context.Context, metadata *M.Metadata) (net.Conn, error) {
	proxy := rr.nextProxy()
	if proxy == nil {
		return nil, errors.New("no healthy proxy available")
	}
	return proxy.DialContext(ctx, metadata)
}

// DialUDP implements proxy.Dialer interface
func (rr *RoundRobinProxy) DialUDP(metadata *M.Metadata) (net.PacketConn, error) {
	proxy := rr.nextProxy()
	if proxy == nil {
		return nil, errors.New("no healthy proxy available")
	}
	return proxy.DialUDP(metadata)
}

// Addr implements proxy.Proxy interface - returns first proxy's address for logging
func (rr *RoundRobinProxy) Addr() string {
	if len(rr.proxies) > 0 {
		return rr.proxies[0].Addr()
	}
	return ""
}

// Proto implements proxy.Proxy interface - returns first proxy's protocol for logging
func (rr *RoundRobinProxy) Proto() proto.Proto {
	if len(rr.proxies) > 0 {
		return rr.proxies[0].Proto()
	}
	return proto.Direct
}
