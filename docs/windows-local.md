# Windows 本机替代天翼客户端运行说明

本文面向把 EsurfingGo 当作 Windows 本机第三方天翼客户端使用的场景。直接双击 `esurfing-windows-amd64.exe` 会打开轻量原生 GUI；需要脚本化运行时仍可使用命令行。

## 1. 查看网卡编号

```powershell
.\esurfing-windows-amd64.exe -s
```

如果电脑同时有以太网、WLAN、虚拟网卡，建议记下实际联网网卡编号，后续写入 `network`。

## 2. 创建本机配置

复制 `esurfing.example.json` 为 `esurfing.local.json`，按需修改：

```json
{
  "user": "18900000000",
  "password": "your-password",
  "network": 2,
  "log_file": "esurfing.log",
  "save_credentials": true,
  "start_with_windows": false
}
```

说明：

- `network` 为 `-s` 输出的网卡编号；填 `0` 表示不强制绑定网卡。
- `esurfing.local.json` 已加入 `.gitignore`，不要提交真实账号密码。

普通校园网登录不需要用户输入短信验证码，GUI 也没有验证码输入框。项目仍保留命令行 `-c` / `-sms` 参数，以兼容少数由门户服务端明确启用短信验证的环境；这不是常规登录步骤。

原始天翼协议中的自动 challenge 属于认证报文内部参数，不是用户手工输入的短信验证码。

## 3. GUI 保存账密、托盘和开机认证

直接双击 exe 会打开轻量原生 GUI。填写账号、密码并选择网卡后：

- 勾选“保存账号密码”会把账号、密码和网卡写入 exe 同目录的 `esurfing.local.json`；取消勾选会清除已保存的账号密码。
- 点击窗口最小化按钮会隐藏到系统托盘；托盘左键恢复窗口，右键可选择“显示主窗口”或“退出”。窗口关闭按钮仍表示退出程序。
- 勾选“开机自动启动并认证”会为当前 Windows 用户设置启动项。该选项要求同时勾选保存账号密码，并且账密不能为空；登录 Windows 后程序会隐藏到托盘并自动开始认证。

配置文件保存为明文，只建议保存到个人电脑本地，不要提交 Git 或同步到网盘。

## 4. 前台试运行

直接双击 exe 会打开 GUI。填写账号、密码并选择网卡后点击“登录并保持”。

```powershell
.\esurfing-windows-amd64.exe -config .\esurfing.local.json
```

看到授权成功后保持窗口打开，程序会自动心跳保活。按 `Ctrl+C` 会发送下线请求并退出。

## 5. 写入日志文件

```powershell
.\esurfing-windows-amd64.exe -config .\esurfing.local.json -log-file .\logs\esurfing.log
```

日志会同时输出到控制台和文件。默认日志不会打印密码、短信验证码、ticket、认证报文正文或 URL 查询参数。

## 6. 命令行计划任务自启（可选）

在 PowerShell 中执行：

```powershell
.\scripts\windows-install-autostart.ps1 `
  -ExePath .\esurfing-windows-amd64.exe `
  -ConfigPath .\esurfing.local.json `
  -LogFilePath .\logs\esurfing.log
```

脚本会注册一个当前用户登录时启动的计划任务 `EsurfingGo`。如果要取消：

```powershell
Unregister-ScheduledTask -TaskName EsurfingGo -Confirm:$false
```

## 7. Clash Verge TUN 联合验证与断网处理

真实开启 TUN 可能改变系统默认路由。验证前先准备一个独立的临时构建目录、日志目录和本地状态采集目录，不要覆盖当前正在运行的 EsurfingGo 正式实例。

### 开启前采集

在本仓库目录执行以下命令；它只读取状态并写入本地文件，不修改网络：

```powershell
.\scripts\capture-network-state.ps1 `
  -OutputDirectory .\tun-test\before `
  -Label before-tun
```

至少保留这些信息：物理网卡名称和 ifIndex、物理 IPv4、默认网关、IPv4 默认路由、DNS、WinHTTP/用户代理设置、Clash 和 EsurfingGo 进程列表。请确认当前物理网卡仍是可用的校园网网卡，并记下其名称和 ifIndex。

### 真实验证步骤

1. 保持正式实例不动，只运行单独构建的临时 EsurfingGo 副本；临时副本使用单独配置和日志文件。
2. 打开 Clash Verge 的 TUN 模式，等待其状态显示为已开启。
3. 立即在 PowerShell 中执行一次采集命令，保存到 `tun-test\tun-on`；网络即使已经断开，该命令也能在本地完成。
4. 启动临时副本，先验证 GUI 打开、刷新网卡、选择物理网卡；再点击登录，观察日志中的 `TUN-safe transport`、DNS、探测、认证和心跳结果。
5. 登录成功或请求失败后，再采集一次到 `tun-test\after-login`，并记录系统是否能访问校园网网关。

示例：

```powershell
.\scripts\capture-network-state.ps1 -OutputDirectory .\tun-test\tun-on -Label tun-on
```

启动临时程序时使用你实际构建出的路径，例如：

```powershell
& "C:\path\to\temporary\esurfing-windows-amd64.exe" `
  -config "C:\path\to\temporary\esurfing.local.json" `
  -log-file "C:\path\to\temporary\tun-test\esurfing.log"
```

### 断网后的恢复

- 首选恢复方式：在 Clash Verge 窗口中关闭 TUN，并等待默认路由回到物理网卡。
- 如果 Clash Verge 主窗口暂时打不开，先不要改路由和 DNS；用本地采集脚本保存现场，然后通过任务管理器结束 Clash Verge 的 Mihomo 核心进程，等待 TUN 路由自动撤销，再重新采集状态。这个应急动作只针对 Clash，不要结束当前 EsurfingGo 正式实例。
- 恢复后执行：

```powershell
.\scripts\capture-network-state.ps1 -OutputDirectory .\tun-test\recovered -Label recovered
```

- 对比 `before` 与 `recovered` 中的 `route.txt`、`ip-interface.txt`、`dns-client.txt` 和 `proxy.txt`。如果默认路由、物理网卡状态或 DNS 没有恢复，不要继续认证测试，手动在系统网络设置中恢复网卡后再处理日志。

本程序的修复目标是只约束自身的 HTTP/TCP/DNS socket 到活动物理 IPv4 网卡，不修改 Clash 配置、系统代理、默认路由或系统 DNS。自动选择模式使用物理 IPv4 源地址绑定，不设置 Windows `IP_UNICAST_IF`；手工选择具体网卡时仍使用强制接口绑定。TUN 持续开启时，程序会在认证请求前检查物理候选网卡、IPv4 地址和 Windows 默认路由优先级；若发现上行切换或地址变化，会关闭旧空闲连接并重建认证绑定。因此联合测试必须分别记录“系统是否断网”和“本程序认证流量是否成功”，不能只看其中一个结果。

这项重建只覆盖 EsurfingGo 自身的认证连接，不覆盖 Clash TUN 的系统路由、热点客户端路由或 Windows ICS 转发；后者仍需要在 Clash/Mihomo 配置和 Windows 网络环境中单独处理。

### 移动热点与 TUN 的能力边界

Windows ICS 常见的移动热点网关是 `192.168.137.1`，另一种常见共享配置是 `192.168.5.1`。程序会只读识别这些网关及其接口 CIDR，并在自动传输启用时记录检测到的热点子网。这些 CIDR 仅用于诊断，不代表加入 `route-exclude-address` 后就能修复热点客户端公网转发；程序不会替用户修改 Clash/Mihomo、路由表、DNS 或 ICS。

如果联合运行时出现断网，先分别判断本机和热点客户端是哪一侧失效：本程序只负责自身认证 socket 的物理网卡隔离，不负责热点客户端公网转发。Windows Mihomo TUN 的整机路由、Windows ICS 共享和热点客户端流量需要单独的转发方案；仅排除热点 CIDR 不能把目的为公网的热点流量自动改走物理网卡。不要在断网现场继续反复修改认证协议，应先保存 `tun-on`、`after-login` 和恢复后的网络状态，再根据 Clash/Mihomo 与 ICS 的日志定位转发链路。

## 当前边界

### 官方客户端抓包

可使用仓库中的 `scripts/capture-process-packets.ps1` 采集官方客户端登录期间的网络数据。脚本优先使用 Windows 自带 `pktmon`，按目标进程当前端口过滤，并同时保存进程信息、连接列表和网络状态；如果系统没有 `pktmon`，可回退到 `netsh trace`。

请以管理员 PowerShell 执行：

```powershell
.\scripts\capture-process-packets.ps1 `
  -ProcessId 23440 `
  -OutputDirectory .\packet-capture-official `
  -DurationSeconds 120
```

运行后立即在官方客户端执行一次登录/重连，等待脚本结束。脚本必须在“管理员 PowerShell”中运行；如果提示 `pktmon failed to start`，请提升 PowerShell 权限后重试。生成的 `official-client.pcapng`、`metadata.json`、`connections-*.txt` 和 `network-state.txt` 保存在指定目录。抓包可能包含账号、密码、票据和会话令牌，请只保存在本机；提交分析前必须脱敏。

- GUI 是轻量原生窗口；命令行/后台模式仍可用于计划任务和脚本化启动。
- 真实账号密码保存在本机 JSON 时是明文；建议只放在个人电脑目录中，并避免同步到网盘或 Git。
- 如果校园网门户页面格式与项目支持的 ESurfing 协议不同，需要保留脱敏日志再做适配。
