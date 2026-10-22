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

模块重启或重新上电后，启用 PIN 的 SIM 会通过下述钥匙串流程自动解锁。eSIM 操作取决于实际卡片，未进行下载或切换验收。仓库没有电话拨号与双向音频功能，通话不属于本次安装已实现的能力。短信发送的后续实机验收见下文。


## SIM PIN 自动恢复（2026-10-22）

当前 USB AT 服务在每次短信轮询和发送前检查 SIM。仅当状态明确为 `SIM PIN`、卡片 ICCID 与钥匙串记录完全一致时尝试一次解锁。READY 不提交 PIN，PUK、PIN2、缺卡、身份不匹配、钥匙串不可用都不提交。

- PIN 和 ICCID 仅存储在 macOS 登录钥匙串：service `local.djonehub.macos`，account `sim-auto-unlock`。服务通过系统 `/usr/bin/security` 读取，不在进程参数、日志或仓库保存明文 PIN。
- 配置或换卡后重新绑定：运行 `python3 scripts/configure-sim-pin.py`，在终端无回显输入 PIN。该脚本仅保存，不测试 PIN。
- 每次提交前，在私有应用目录写入 `pin-unlock.pending` 并同步到磁盘。失败、超时或服务异常退出后保留标记，防止服务重启继续消耗 PIN 次数。确认解锁为 READY 后才删除标记。
- 如果保护被触发，先人工核对 SIM 状态、PIN 和剩余尝试次数；确认问题已解决后再人工移除 `~/Library/Application Support/DJOneHub/pin-unlock.pending`。配置脚本不会自动移除此保护。
- 登录钥匙串锁定时自动解锁暂停，恢复钥匙串访问后下一轮重试读取凭据。无需关闭 SIM 的 PIN 保护。
- 模块重启后，短信服务初始化可能比 SIM READY 晚一些；轮询会继续等待，不会把 AT ERROR 当成成功读取空收件箱。

验收：从 PIN 锁定状态安装启动后自动恢复；分别切到 USB 模式 1、模式 0 并重启，均自动完成 PIN 解锁、网络注册及无错误短信轮询。最后保留模式 0。原始短信备份位于当前用户私有应用目录的 `backups/`，不进入 Git。短信存储满会影响接收，自动删除仍关闭。

## 短信发送结果

发送过程中禁止页面重复点击和服务端并行提交；支持没有 submitter 的键盘提交。发送结果显示在表单下方，刷新页面可读取服务端本次运行的最后发送状态。该状态不保存号码或正文，服务重启后清空。

只有收到 `+CMGS` 和 `OK` 才显示“已提交到网络”；模块错误保留 CMS/CME 错误码；连接中断、超时或确认不完整显示“结果不确定”，不会自动重发。长短信中途失败会提示已经提交的分片数，避免整条重复发送。浏览器请求超过 120 秒后停止等待，但后台操作可能仍在执行，应等待服务端状态更新。

经用户单次授权，实机提交一条测试短信，返回一个分片提交成功；未验证接收方最终送达或业务处理。页面旧错误为设置 PDU 模式失败，和模块刚重启、SIM/短信服务未就绪的情况一致；这不是运营商已拒收的证据。

验证命令：

```sh
go test -race ./cmd/djonehub-macos ./pkg/smscodec
node scripts/tests/sms-ui.cjs
node --check cmd/djonehub-macos/web/app.js
```
