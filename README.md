# EsurfingGo

中国电信天翼校园网（ESurfing）第三方认证客户端。项目使用 Go 编写，Windows 版本提供轻量原生 GUI，可作为本机天翼客户端的替代品；命令行版本同时适合 Linux、macOS、服务器和路由器部署。

> 当前正式版本：`v1.2.1`
>
> 这是第三方实现，不是中国电信官方软件。不同学校的门户配置可能存在差异，首次使用请保留脱敏日志。

## 功能概览

- 自动检测强制门户（Captive Portal）并完成认证
- 自动心跳保活，断线自动重连
- 跨平台编译（Windows / Linux / macOS）
- Go编译后为单文件且无须依赖，便于路由器部署
- 支持多拨
- Windows 轻量原生 GUI，不依赖 Qt、Electron 或额外运行时
- 选择、刷新网络接口，避免认证流量进入 Clash/Mihomo 等虚拟网卡
- 最小化到系统托盘，托盘菜单支持恢复窗口和退出
- 可选保存账号密码和网卡选择；本地配置为明文 JSON
- 可选当前用户开机启动，并使用已保存账号密码自动认证
- Clash Verge TUN 开启时，针对 Fake-IP 门户探测做了专门兼容
- 单文件发布，支持 Windows、Linux、macOS 以及常见路由器架构
- 支持命令行预填短信验证码参数，以兼容少数明确要求短信验证的门户

## Windows GUI

### 直接使用

从 [Releases](https://github.com/ljy87263621/EsurfingGo/releases/latest) 下载 `EsurfingGo-v1.2.1-windows-amd64.zip`，解压到一个固定目录后双击 `esurfing-windows-amd64.exe`。Windows 10/11 原生系统即可运行，不需要安装 Go、Python、Qt 或其他运行库。

Windows GUI 和系统托盘使用项目内嵌的 `assets/esurfing.ico` 图标，Portable 压缩包不需要额外携带图标文件。

GUI 的主要控件如下：

| 控件 | 作用 |
| --- | --- |
| 账号、密码 | 填写校园网认证凭据；密码输入框会以密码形式显示 |
| 网卡下拉框 | 选择认证使用的网卡；“自动选择”由程序按系统路由处理 |
| 刷新网卡 | 重新读取当前可用的物理 IPv4 网卡，不阻塞窗口 |
| 登录并保持 | 启动检测、认证和心跳保活 |
| 停止/注销 | 停止客户端循环，并在已登录时发送注销请求 |
| 保存账号密码 | 保存账号、密码和网卡选择到 exe 同目录的 `esurfing.local.json` |
| 开机自动启动并认证 | 写入当前用户启动项；登录 Windows 后隐藏到托盘并自动认证 |
| 窗口最小化 | 隐藏到系统托盘，不停止认证；托盘左键恢复，右键打开菜单 |

开机自动认证要求同时满足以下条件：

1. 勾选“保存账号密码”；
2. 账号和密码均不为空；
3. exe 位于一个登录用户可访问的固定目录。

配置文件中的密码是明文，仅建议在个人电脑本地使用，不要提交 Git、同步网盘或发送给他人。取消“保存账号密码”后，GUI 会清除配置文件中的账号和密码。

### GUI 使用建议

1. 第一次启动先点击“刷新网卡”。
2. 选择实际承载校园网连接的“以太网”或“WLAN”，不要选择 Clash、Mihomo、Wintun、WireGuard、TAP 等虚拟接口。
3. 填写账号和密码，先点击“登录并保持”观察日志。
4. 确认认证和心跳稳定后，再按需勾选保存账密、托盘运行或开机自动认证。

GUI 不提供手工短信验证码输入框。普通校园网认证通常不需要用户输入验证码；命令行仍保留兼容参数，详见下文。协议内部的自动 `challenge` 是认证报文参数，不等于用户收到的短信验证码。

## Clash Verge TUN 兼容说明

开启 Clash Verge 的 TUN 模式后，系统 DNS 可能把 `connect.rom.miui.com` 解析为 `198.18.x.x` 或 `198.19.x.x` Fake-IP。若认证流量已经绑定到物理网卡，直接连接这个地址会导致门户探测超时，即使 Clash 代理本身仍然可以上网。

v1.2.1 对这个场景做了分层处理：

- 自动排除常见 Clash/Mihomo/Wintun/TUN/TAP 等虚拟网卡，将认证 HTTP/TCP 流量绑定到可用物理 IPv4 网卡。
- 自动选择模式只绑定物理 IPv4 源地址，不再对 Windows socket 设置 `IP_UNICAST_IF`；这样可以降低 EsurfingGo 认证连接与 Windows ICS/WFP 的耦合。手工选择具体网卡时仍保留强制接口绑定。
- TUN 持续开启期间会检查物理候选网卡、IPv4 地址和 Windows 默认路由优先级；发现上行切换或地址变化时，会关闭旧空闲连接并重建认证绑定。
- 仅给门户探测请求增加专用处理：通过当前系统代理访问 DoH，获取真实 IPv4，并过滤 `198.18.0.0/15` Fake-IP。
- 连接真实 IP 时保留原始 `Host`，因此仍按门户域名访问。
- Ticket、Auth、Heartbeat、注销等认证请求仍走物理网卡绑定的原有传输，不会把“代理能联网”误判为“校园网已认证”。
- 程序不会修改 Clash 配置、系统代理、默认路由或系统 DNS。

这里的动态重建只作用于 EsurfingGo 自己的认证连接，不能修复 Clash TUN 对系统流量、热点客户端流量或 Windows ICS 转发的错误路由。

### TUN 与 Windows 移动热点同时开启

Windows 移动热点通常使用 `192.168.137.0/24`，部分系统或共享配置会使用 `192.168.5.0/24`。程序会在选择认证物理网卡时排除常见热点接口，并识别这两个常见 ICS 网关地址；启动日志会输出检测到的热点子网，但这些 CIDR 仅用于诊断，不代表加入 `route-exclude-address` 后就能修复热点客户端公网转发。

在 TUN 与移动热点同时开启时，本程序只能保证自己的认证流量尝试绑定物理上行，不能替代 Clash/Mihomo 对系统路由，也不能实现 Windows ICS 的 VPN 共享。热点客户端的公网连接仍由 Clash TUN、Windows ICS 和底层转发路径共同决定；当前 Mihomo Windows TUN 也不等同于 VPNHotspot 类共享方案。因此出现本机或热点客户端断网时，应将其视为联合网络架构问题，而不是继续修改 EsurfingGo 认证协议。不要让 EsurfingGo 自动改写 Clash 配置、系统路由、DNS 或代理配置。

日志中的 `HTTP 204` 表示门户探测确认可以访问公网；只有随后出现认证成功、登录保持或心跳成功日志时，才表示校园网认证流程完成。

如果 TUN 开启后导致整机断网，优先在 Clash Verge 中关闭 TUN，等待路由恢复后再继续操作。正式实例不要反复启动多个副本；测试 TUN 时请使用单独目录、配置和日志。

## 命令行使用

```bash
esurfing -u <用户名> -p <密码> [-c <短信验证码>] [-n <网络接口号>]
esurfing -config <配置文件> [-log-file <日志文件>]
```

### 参数

| 参数                   | 说明                   |
| ---------------------- | ---------------------- |
| `-u` / `-user`     | 登录用户名             |
| `-p` / `-password` | 登录密码               |
| `-c` / `-sms`      | 可选的短信验证兼容参数；普通校园网登录无需填写，GUI 不提供该输入项 |
| `-s` / `--show`    | 查看网络接口          |
| `-n` / `--network` | 使用指定网络接口认证   |
| `-config`           | 从 JSON 配置文件读取账号、密码、网卡、日志配置 |
| `-log-file`         | 同时把日志追加写入指定文件 |

### 示例

```bash
# 基本登录
esurfing -u 13800138000 -p mypassword

# 仅在门户明确要求短信验证时使用
esurfing -u 13800138000 -p mypassword -c 123456

#指定特定网络接口，如wifi进行拨号
esurfing  -s
#返回
1：以太网
2：WLAN

esurfing -n 2 -u 13800138000 -p mypassword

# Windows 本机长期运行可使用配置文件
esurfing -config .\esurfing.local.json -log-file .\logs\esurfing.log

```

程序启动后会自动检测网络状态，完成认证并保持连接。按 `Ctrl+C` 安全退出。

## 部署方法

### Windows 本机替代客户端

Windows 用户优先使用上面的 Portable GUI。需要脚本化运行时，可以查看网卡并使用配置文件：

```powershell
.\esurfing-windows-amd64.exe -s
Copy-Item .\esurfing.example.json .\esurfing.local.json
.\esurfing-windows-amd64.exe -config .\esurfing.local.json -log-file .\logs\esurfing.log
```

GUI 账密保存、托盘和开机认证的完整说明见 [`docs/windows-local.md`](docs/windows-local.md)。也可以使用其中的 PowerShell 脚本注册当前用户计划任务。

### Linux Systemd

创建服务文件 `/etc/systemd/system/esurfing.service`：

````ini
[Unit]
Description=EsurfingGo Campus Network Authenticator
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/esurfing -config /etc/esurfing/esurfing.json
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
````

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable esurfing #开机自启
sudo systemctl start esurfing #启动程序
sudo systemctl status esurfing #查看状态
```

### OpenWrt / DD-WRT / 梅林

将对应架构的二进制上传到路由器，例如 `/usr/bin/esurfing`，添加执行权限后，在固件提供的启动脚本中运行：

```bash
chmod +x /usr/bin/esurfing
/usr/bin/esurfing -u <用户名> -p <密码> >> /tmp/esurfing.log 2>&1 &
```

路由器部署时请使用权限受限的配置文件，避免把明文密码暴露给普通用户或日志系统。

## 构建方法

### 本机构建

```bash
go build -trimpath -ldflags="-s -w" -o esurfing .
```

Windows 原生 GUI 使用默认构建入口，不依赖 Qt、Electron、.NET Runtime 或其他额外运行库。

### 跨平台构建

PowerShell 脚本会构建 Windows、Linux、macOS 以及常见路由器架构，输出到 `bin/`：

```powershell
.\build.ps1
```

Linux/macOS 可使用：

```bash
chmod +x build.sh
./build.sh
```

也可以手动构建 ARMv5：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -trimpath -ldflags="-s -w" -o esurfing-linux-armv5 .
```

正式发布的 Windows 资产为 `EsurfingGo-v1.2.1-windows-amd64.zip`，包含可执行文件、README、许可证、配置模板和 Windows 本机说明；无需用户安装 Go 或其他运行环境。

## 测试验证

提交或发布前运行：

```bash
gofmt -w *.go network/*.go cipher/*.go utils/*.go model/*.go
go test ./... -count=1
go test -race ./...
go vet ./...
git diff --check
```

Windows 交叉编译检查：

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go test ./... -run '^$' -count=1
go build -trimpath -ldflags='-H=windowsgui -s -w' -o dist\EsurfingGo-v1.2.1\esurfing-windows-amd64.exe .
```

Windows 图标资源已经以 `esurfing-resource_windows_amd64.syso` 提交到仓库。若替换 `assets/esurfing.ico`，可在仓库根目录用 MinGW `windres` 重新生成：

```powershell
windres --target=pe-x86-64 -i assets\esurfing.rc -o esurfing-resource_windows_amd64.syso
```

## 项目结构

```text
main.go                 # CLI 入口和 Windows GUI/CLI 模式选择
client.go               # 认证客户端主逻辑
config.go               # JSON 配置和日志文件配置
gui_windows.go          # Windows 原生 GUI、托盘和异步操作
gui_common.go            # GUI 共用状态和更新逻辑
assets/esurfing.ico      # Windows GUI 和托盘图标
assets/esurfing.rc       # Windows 图标资源描述
autostart_windows.go     # 当前用户开机启动
iface.go                 # 网络接口筛选与物理网卡绑定
network/                 # HTTP、门户探测、TUN 兼容传输
cipher/                  # AES、3DES、SM4、ZUC 等协议算法
model/                   # 协议数据模型
utils/                   # 通用工具函数
docs/windows-local.md    # Windows 本机运行和断网恢复说明
scripts/                 # 本地网络采集与计划任务辅助脚本
```

## 依赖

- [gmsm](https://github.com/emmansun/gmsm) - 国密 SM4 / ZUC 算法
- [google/uuid](https://github.com/google/uuid) - UUID 生成
- Windows GUI 使用 Win32 原生 API，不依赖 Qt、Electron 或 .NET Runtime

## 许可证与来源

本项目使用 [MIT License](LICENSE)。项目结构与协议解析逻辑继承自 [Rsplwe/EsurfingDialer](https://github.com/Rsplwe/EsurfingDialer)，并在此基础上维护 Go 版本、Windows GUI、本地配置、开机启动和 TUN 兼容逻辑。

使用本软件前，请确认符合所在学校网络和当地法律法规。项目维护者不对校园网策略变化、账号安全或网络中断承担责任。
