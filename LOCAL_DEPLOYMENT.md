# macOS 源码部署

本分支基于 ZenGeekLabs/DJOneHub 的 `f7f1a0dda3eda6c40585d93a4654bf94d50b8b08`。

## 本地调整

- 禁用读取短信后的自动 ME 清理。上游缓存位于内存，自动删除模块原短信会导致服务重启后丢失记录。仍保留页面中的手动清理功能；模块存储接近满时应先备份再手动清理。
- 补齐文档引用的 `scripts/install-macos.sh`，使用当前用户的 LaunchAgent，无需 root，登录后启动、异常退出后重启，只监听 `127.0.0.1:7575`。
- PIN、SIM 标识、短信正文及运行日志均不进入 Git。

## 构建与安装

需要 Go 1.26.3、Python 3、C 编译工具及 pkg-config 可找到的 libusb 1.0。Apple Silicon Homebrew 的 libusb 位于 `/opt/homebrew`，本地构建的程序依赖该运行库。

```sh
go test ./cmd/djonehub-macos ./pkg/smscodec
sh scripts/build-macos.sh
sh scripts/install-macos.sh
```

访问 <http://127.0.0.1:7575>。

安装位置：

- 程序：`~/Library/Application Support/DJOneHub/djonehub-macos`
- 自启动：`~/Library/LaunchAgents/local.djonehub.macos.plist`
- 日志：`~/Library/Logs/DJOneHub/service.log`

日志和应用目录仅允许当前用户访问。服务运行期间缓存短信；关闭自动清理后，重启会从模块重读仍保留的短信。日志目前没有自动轮转。

## 服务管理

```sh
# 状态
launchctl print gui/$(id -u)/local.djonehub.macos
# 重启
launchctl kickstart -k gui/$(id -u)/local.djonehub.macos
# 停止并卸载本次登录会话中的服务
launchctl bootout gui/$(id -u) "$HOME/Library/LaunchAgents/local.djonehub.macos.plist"
```

永久取消登录自启动时，停止后将该 plist 移出 `~/Library/LaunchAgents`。重新安装可恢复服务。

## 实机验收与边界

2026-09-12：原生服务通过 USB AT 连接真实模块，SIM 已就绪并注册 LTE；网页设备状态和网络页面正常，短信接口成功解析已有短信，自动轮询无错误，自动清理关闭。长短信会合并，因此页面消息数可能少于模块存储的分片数。

后续完成 USB 网卡配置：设置 `usbnet=1` 并重启模块，SIM 按需输入 PIN 解锁，macOS 自动通过 DHCP 获取地址和模块提供的 DNS。将 `EG25G-QDC507` 网络服务排在 Wi-Fi 前，其他服务相对顺序不变。

已通过绑定 USB 网卡的 HTTPS 请求验证 4G 链路；再通过默认路由及不绑定网卡、不走代理的 HTTPS 请求验证 Mac 默认出口为模块，均返回 HTTP 200。模块 DNS 查询成功。网卡模式下仍可通过 USB AT 读取 SIM、注册状态和短信。

如需改回 Wi-Fi 优先，在 macOS 网络设置的服务顺序中将 Wi-Fi 移到模块之前即可，不需要关闭模块。切回模式 0 会使 USB 网卡消失；重启模块可能重新要求 SIM PIN。DHCP 获得地址本身不能证明蜂窝链路已联网。

模块重启或重新上电后，启用 PIN 的 SIM 可能需要重新解锁；本安装不保存 PIN、不自动尝试解锁。eSIM 操作取决于实际卡片，未进行下载或切换验收。仓库没有电话拨号与双向音频功能，通话不属于本次安装已实现的能力。短信发送入口已提供，未向外部号码发送测试消息。
