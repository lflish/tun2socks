![tun2socks](docs/logo.png)

[![GitHub Workflow][1]](https://github.com/xjasonlyu/tun2socks/actions)
[![Go Version][2]](https://github.com/xjasonlyu/tun2socks/blob/main/go.mod)
[![Go Report][3]](https://goreportcard.com/badge/github.com/xjasonlyu/tun2socks)
[![Maintainability][4]](https://codeclimate.com/github/xjasonlyu/tun2socks/maintainability)
[![GitHub License][5]](https://github.com/xjasonlyu/tun2socks/blob/main/LICENSE)
[![Docker Pulls][6]](https://hub.docker.com/r/xjasonlyu/tun2socks)
[![Releases][7]](https://github.com/xjasonlyu/tun2socks/releases)

> [English README](README.md) | [原始项目](https://github.com/xjasonlyu/tun2socks)

## ✨ 此分支的新特性

本分支在原始 [xjasonlyu/tun2socks](https://github.com/xjasonlyu/tun2socks) 基础上扩展了**多服务器负载均衡**功能：

- 🔄 **轮询负载均衡**: 自动在多个代理服务器之间分配连接
- 📈 **更好的性能**: 通过多代理提升吞吐量并降低延迟
- 🛡️ **增强冗余**: 服务器不可用时自动故障转移
- 🔧 **向后兼容**: 现有的单代理配置继续无变化工作
- 📝 **灵活配置**: 支持 YAML 数组和单字符串代理配置

## 功能特性

- **通用代理**: 透明地将任何应用程序的所有网络流量通过代理路由
- **多协议支持**: 支持 HTTP/SOCKS4/SOCKS5/Shadowsocks 代理及可选身份验证
- **多服务器负载均衡**: 支持多个代理服务器的自动轮询负载均衡
- **跨平台**: 在 Linux/macOS/Windows/FreeBSD/OpenBSD 上运行，具有平台特定优化
- **网关模式**: 作为第3层网关路由同一网络上其他设备的流量
- **完整 IPv6 兼容性**: 原生支持 IPv6；无缝地在 IPv4 和 IPv6 之间隧道传输
- **用户空间网络**: 利用 **[gVisor](https://github.com/google/gvisor)** 网络栈增强性能和灵活性

## 性能基准

![benchmark](docs/benchmark.png)

在所有使用场景中，tun2socks 性能表现最佳。
更多详情请参见 [基准测试](https://github.com/xjasonlyu/tun2socks/wiki/Benchmarks)。

## 配置说明

### 多服务器负载均衡

tun2socks 支持多个代理服务器的自动轮询负载均衡。您可以通过两种方式配置多个代理：

#### 命令行使用
```bash
# 单个代理（向后兼容）
./tun2socks -device tun0 -proxy socks5://127.0.0.1:1080

# 多个代理使用 YAML 配置文件
./tun2socks -device tun0 -config config.yaml
```

#### YAML 配置
```yaml
# 单个代理（字符串格式）
proxy: socks5://127.0.0.1:1080

# 多个代理（数组格式）- 自动轮询负载均衡
proxy:
  - socks5://127.0.0.1:1080
  - socks5://127.0.0.1:1081

# 其他配置选项
device: tun0
mtu: 1500
loglevel: info
```

当配置多个代理时，tun2socks 将使用轮询负载均衡自动在所有服务器之间分配连接。这提供了更好的性能和冗余性。

## 文档

- [从源码安装](https://github.com/xjasonlyu/tun2socks/wiki/Install-from-Source)
- [快速开始示例](https://github.com/xjasonlyu/tun2socks/wiki/Examples)
- [内存优化](https://github.com/xjasonlyu/tun2socks/wiki/Memory-Optimization)

完整的文档和技术指南可在 [Wiki](https://github.com/xjasonlyu/tun2socks/wiki) 中找到。

## 社区

欢迎在 [讨论区](https://github.com/xjasonlyu/tun2socks/discussions) 提出任何问题。

## 致谢

- [google/gvisor](https://github.com/google/gvisor) - 容器应用内核
- [wireguard-go](https://git.zx2c4.com/wireguard-go) - WireGuard 的 Go 实现
- [wintun](https://git.zx2c4.com/wintun/) - Windows 第3层 TUN 驱动程序

## 许可证

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fxjasonlyu%2Ftun2socks.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fxjasonlyu%2Ftun2socks?ref=badge_large)

从 `v2.6.0` 开始的所有版本均在 [MIT 许可证](https://github.com/xjasonlyu/tun2socks/blob/main/LICENSE) 条款下提供。

## Star 历史

<a href="https://star-history.com/#xjasonlyu/tun2socks&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=xjasonlyu/tun2socks&type=Date&theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=xjasonlyu/tun2socks&type=Date" />
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=xjasonlyu/tun2socks&type=Date" />
  </picture>
</a>

[1]: https://img.shields.io/github/actions/workflow/status/xjasonlyu/tun2socks/docker.yml?logo=github

[2]: https://img.shields.io/github/go-mod/go-version/xjasonlyu/tun2socks?logo=go

[3]: https://goreportcard.com/badge/github.com/xjasonlyu/tun2socks

[4]: https://api.codeclimate.com/v1/badges/b5b30239174fc6603aca/maintainability

[5]: https://img.shields.io/github/license/xjasonlyu/tun2socks

[6]: https://img.shields.io/docker/pulls/xjasonlyu/tun2socks?logo=docker

[7]: https://img.shields.io/github/v/release/xjasonlyu/tun2socks?logo=smartthings