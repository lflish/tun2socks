package engine

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/log"
	"github.com/xjasonlyu/tun2socks/v2/metadata"
	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	config         HealthCheckConfig
	healthyProxies map[string]proxy.Proxy // 健康的代理列表
	allProxies     map[string]proxy.Proxy // 所有代理列表
	mu             sync.RWMutex
	stopCh         chan struct{}
	updateCallback func([]proxy.Proxy) // 更新回调函数
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker(config HealthCheckConfig, proxies []proxy.Proxy, updateCallback func([]proxy.Proxy)) *HealthChecker {
	hc := &HealthChecker{
		config:         config,
		healthyProxies: make(map[string]proxy.Proxy),
		allProxies:     make(map[string]proxy.Proxy),
		stopCh:         make(chan struct{}),
		updateCallback: updateCallback,
	}

	// 设置默认值
	if hc.config.Interval == 0 {
		hc.config.Interval = 30 * time.Second
	}
	if hc.config.Timeout == 0 {
		hc.config.Timeout = 5 * time.Second
	}
	if hc.config.URL == "" {
		hc.config.URL = "http://www.google.com"
	}

	// 初始化代理列表
	for _, p := range proxies {
		key := fmt.Sprintf("%s://%s", p.Proto(), p.Addr())
		hc.allProxies[key] = p
		hc.healthyProxies[key] = p // 初始时假设所有代理都是健康的
	}

	return hc
}

// Start 启动健康检查器
func (hc *HealthChecker) Start() {
	if !hc.config.Enable {
		log.Infof("[HEALTH_CHECKER] 健康检查已禁用")
		return
	}

	log.Infof("[HEALTH_CHECKER] 启动健康检查器，检查间隔: %v, 超时: %v, 目标URL: %s",
		hc.config.Interval, hc.config.Timeout, hc.config.URL)

	go hc.run()
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() {
	if hc.stopCh != nil {
		close(hc.stopCh)
	}
}

// GetHealthyProxies 获取健康的代理列表
func (hc *HealthChecker) GetHealthyProxies() []proxy.Proxy {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var proxies []proxy.Proxy
	for _, p := range hc.healthyProxies {
		proxies = append(proxies, p)
	}
	return proxies
}

// run 执行健康检查循环
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	// 立即执行一次检查
	hc.checkAllProxies()

	for {
		select {
		case <-ticker.C:
			hc.checkAllProxies()
		case <-hc.stopCh:
			return
		}
	}
}

// checkAllProxies 检查所有代理的健康状态
func (hc *HealthChecker) checkAllProxies() {
	log.Debugf("[HEALTH_CHECKER] 开始检查 %d 个代理服务器", len(hc.allProxies))

	var wg sync.WaitGroup
	newHealthyProxies := make(map[string]proxy.Proxy)
	var mu sync.Mutex

	for key, p := range hc.allProxies {
		wg.Add(1)
		go func(key string, proxy proxy.Proxy) {
			defer wg.Done()

			if hc.checkProxy(proxy) {
				mu.Lock()
				newHealthyProxies[key] = proxy
				mu.Unlock()
				log.Debugf("[HEALTH_CHECKER] 代理 %s 健康检查通过", key)
			} else {
				log.Warnf("[HEALTH_CHECKER] 代理 %s 健康检查失败", key)
			}
		}(key, p)
	}

	wg.Wait()

	// 更新健康代理列表
	hc.mu.Lock()
	oldCount := len(hc.healthyProxies)
	hc.healthyProxies = newHealthyProxies
	newCount := len(hc.healthyProxies)
	hc.mu.Unlock()

	if oldCount != newCount {
		log.Infof("[HEALTH_CHECKER] 健康代理数量变化: %d -> %d", oldCount, newCount)
	}

	// 如果没有健康的代理，警告用户但保留所有代理以防止完全断线
	if newCount == 0 {
		log.Errorf("[HEALTH_CHECKER] 警告：所有代理都不健康，保留原有代理列表以防止断线")
		hc.mu.Lock()
		for key, p := range hc.allProxies {
			hc.healthyProxies[key] = p
		}
		hc.mu.Unlock()
	}

	// 调用更新回调
	if hc.updateCallback != nil {
		hc.updateCallback(hc.GetHealthyProxies())
	}
}

// checkProxy 检查单个代理的健康状态
func (hc *HealthChecker) checkProxy(proxy proxy.Proxy) bool {
	// 解析目标URL
	targetURL, err := url.Parse(hc.config.URL)
	if err != nil {
		log.Errorf("[HEALTH_CHECKER] 无法解析目标URL %s: %v", hc.config.URL, err)
		return false
	}

	// 获取目标主机和端口
	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		if targetURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// 解析IP地址
	ips, err := net.LookupIP(host)
	if err != nil {
		log.Debugf("[HEALTH_CHECKER] 无法解析域名 %s: %v", host, err)
		return false
	}
	if len(ips) == 0 {
		log.Debugf("[HEALTH_CHECKER] 域名 %s 没有解析到IP地址", host)
		return false
	}

	// 转换端口为uint16
	portNum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		log.Debugf("[HEALTH_CHECKER] 无效的端口号 %s: %v", port, err)
		return false
	}

	// 创建元数据
	var dstIP netip.Addr
	if ip4 := ips[0].To4(); ip4 != nil {
		dstIP = netip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], ip4[3]})
	} else if ip6 := ips[0].To16(); ip6 != nil {
		var ip6Array [16]byte
		copy(ip6Array[:], ip6)
		dstIP = netip.AddrFrom16(ip6Array)
	} else {
		log.Debugf("[HEALTH_CHECKER] 无法处理IP地址格式: %v", ips[0])
		return false
	}

	metadata := &M.Metadata{
		Network: metadata.TCP,
		DstIP:   dstIP,
		DstPort: uint16(portNum),
	}

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	// 通过代理建立连接
	conn, err := proxy.DialContext(ctx, metadata)
	if err != nil {
		log.Debugf("[HEALTH_CHECKER] 代理 %s://%s 连接失败: %v", proxy.Proto(), proxy.Addr(), err)
		return false
	}
	defer conn.Close()

	// 发送HTTP请求
	if err := hc.sendHTTPRequest(conn, targetURL); err != nil {
		log.Debugf("[HEALTH_CHECKER] 代理 %s://%s HTTP请求失败: %v", proxy.Proto(), proxy.Addr(), err)
		return false
	}

	return true
}

// sendHTTPRequest 发送HTTP请求
func (hc *HealthChecker) sendHTTPRequest(conn net.Conn, targetURL *url.URL) error {
	// 设置连接超时
	conn.SetDeadline(time.Now().Add(hc.config.Timeout))

	// 构造HTTP请求
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
		targetURL.Path, targetURL.Host)
	if targetURL.Path == "" {
		request = fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
			targetURL.Host)
	}

	// 发送请求
	_, err := conn.Write([]byte(request))
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %v", err)
	}

	// 读取响应
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("读取HTTP响应失败: %v", err)
	}

	// 简单检查是否收到HTTP响应
	response := string(buffer)
	if len(response) > 0 && (contains(response, "HTTP/1.1") || contains(response, "HTTP/1.0")) {
		return nil
	}

	return fmt.Errorf("无效的HTTP响应")
}

// contains 检查字符串是否包含子字符串（简单实现避免引入strings包）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexSubstring(s, substr) >= 0
}

// indexSubstring 查找子字符串位置
func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
