# Trojan 多用户管理平台 (Enhanced Edition)

[![GitHub Release](https://img.shields.io/github/v/release/Level6me/trojan.svg?style=flat-square)](https://github.com/Level6me/trojan/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Level6me/trojan?style=flat-square)](https://goreportcard.com/report/github.com/Level6me/trojan)
[![License](https://img.shields.io/badge/License-GPL%20v3-blue.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20(x86__64%20%7C%20arm64)-orange.svg?style=flat-square)]()

现代化、全功能、轻量级的 **Trojan / Trojan-Go 多用户管理部署平台**。提供极具美感的 Glassmorphism 现代 Web 控制台与高效的 Linux CLI 命令行双重管理能力。

> 💡 **特别声明与致谢**：  
> 本项目基于原著名开源项目 [**Jrohy/trojan**](https://github.com/Jrohy/trojan) 深度重构与二次开发，在传承其轻量稳定的系统架构基础上，全面重构了现代化 Web 前端、多维用户画像详情卡片、实时网络速率采样、真实客户端 IP 自动嗅探记录、用户独立限速、到期时间管理与 CSV 批量导入等生产级功能。向原作者及开源社区致以由衷的敬意与感谢！

---

## 🌟 核心特性与升级亮点

### 🎨 1. 现代化 Glassmorphism 交互控制台
- **极速响应**：原生轻量架构，零冗余依赖，秒级加载。
- **深浅主题自适应**：原生适配系统深色/浅色外观模式，视觉细腻精致。
- **全端适配**：深度优化移动端与桌面端视口，彻底消除误触缩放，操作流畅自然。

### 👤 2. 全维度用户画像详情卡片
- **凭据管理**：查看/修改节点密码、一键生成 12 位高强度随机密码、一键复制。
- **实时流量与配额监控**：上传/下载流量双向明细、实时配额进度条、配额设定与一键流量重置。
- **传输速率动态统计**：自适应动态采样并持久化记录用户的**历史最高下载速率**与**最高上传速率**。
- **安全审计与 IP 嗅探**：内置底层日志嗅探监听器（Log Watcher），无需修改 Trojan 原生内核即可从认证事件流中**自动提取并记录客户端真实历史连接 IP** 与最后活跃时间。
- **灵活有效期管理**：可视化日期选择器，支持 `+30天`、`+90天`、`+半年`、`+1年`、`设为永久` 快捷预设与到期自动停用。
- **用户独立限速策略**：支持单用户 MB/s 级别独立带宽限速（支持 `2/5/10/20/50 MB/s` 快捷预设）。

### 📈 3. 图表可视化与历史明细
- **个人历史用量明细**：集成 Chart.js 走势图，呈现个人近 30 天每日用量柱状图与明细列表。
- **全站用量分析**：全站近 24 小时整点流量走势图、近 7 天每日流量汇总及今日流量排行榜 TOP5。

### 📤 4. CSV 批量导入与导出
- **智能表头识别**：支持带表头/无表头智能匹配，自适应识别字节或 GB 配额单位。
- **增量自动同步（Upsert）**：已存在相同用户名的用户自动覆盖更新配置，密码自动计算 SHA-224 哈希。
- **完整字段导出**：支持导出包含密码、配额、到期时间、限速、历史 IP 等全部维度的 UTF-8 BOM CSV 文件。

### 🛰️ 5. 核心与系统生态
- **双内核热切换**：支持在 Trojan C++ 原生内核 与 Trojan-Go 之间随时无缝热切换。
- **TLS 证书全自动管理**：集成 ACME 证书一键申请，并配置系统定时任务自动无缝续期。
- **多客户端便捷分发**：一键生成 `trojan://` 节点链接、二维码扫码极速导入、Clash 智能订阅生成。
- **CLI 命令行补全**：提供完备的终端命令行管理模式，支持 Bash / Zsh 自动补全。

---

## 🚀 极速安装与部署

> ⚠️ **前置要求**：请提前准备好解析至当前服务器公网 IP 的可用域名。

### a. 一键部署安装（推荐）

以 `root` 用户登录服务器，执行以下一键安装命令：

```bash
curl -sSL https://raw.githubusercontent.com/Level6me/trojan/main/install.sh | bash
```

安装完成后：
- 终端中直接输入 `trojan` 即可呼出交互式命令行管理菜单。
- 浏览器访问 `https://您的域名:8085` 或 `https://您的域名` 即可进入 Web 管理面板。

---

### b. 一键极速更新

当有新版本发布时，直接运行以下命令即可实现**无缝平滑升级**（自动备份、升级数据库表结构与配置并重启服务）：

```bash
curl -sSL https://raw.githubusercontent.com/Level6me/trojan/main/update.sh | bash
```

或者在已安装的服务器终端中直接执行：
```bash
trojan updateWeb
```

---

### c. 一键彻底卸载

如需彻底移除 Trojan 核心、管理面板及专用数据库，运行：

```bash
curl -sSL https://raw.githubusercontent.com/Level6me/trojan/main/install.sh | bash -s -- --remove
```

---

## 🐳 Docker 部署运行

### 1. 运行 MariaDB 数据库容器
```bash
docker run --name trojan-mariadb   --restart=always   -p 3306:3306   -v /home/mariadb:/var/lib/mysql   -e MYSQL_ROOT_PASSWORD=trojan   -e MYSQL_ROOT_HOST=%   -e MYSQL_DATABASE=trojan   -d mariadb:10.2
```

### 2. 运行 Trojan 容器
```bash
docker run -it -d   --name trojan   --net=host   --restart=always   --privileged   level6me/trojan init
```
进入容器初始化：`docker exec -it trojan bash` 后执行 `trojan` 即可配置。

---

## ⌨️ CLI 命令行操作指南

终端中直接输入 `trojan` 可进入交互式控制菜单，也可使用以下子命令快速操作：

```bash
Usage:
  trojan [flags]
  trojan [command]

Available Commands:
  add           添加用户 (支持指定密码、流量配额、有效期)
  clean         清空指定用户已消耗流量
  del           删除指定用户
  info          查看所有用户信息列表及流量状态
  port          修改 Trojan 服务端监听端口
  tls           TLS 证书在线申请与安装 (基于 acme.sh)
  log           查看 Trojan 服务端实时运行日志
  start         启动 Trojan 核心服务
  stop          停止 Trojan 核心服务
  restart       重启 Trojan 核心服务
  status        查看 Trojan 运行状态与在线时长
  update        更新 Trojan 核心内核 (支持指定版本号)
  updateWeb     更新 Trojan 管理程序至最新版本
  upgrade       平滑升级数据库表结构与配置文件
  version       显示当前程序版本号
  web           以 Web 管理面板模式启动 (默认端口: 8085)
  import [file] 批量导入 CSV / SQL 数据文件
  export [file] 导出用户数据至 CSV / SQL 文件
  completion    生成 Shell 自动命令补全脚本 (Bash/Zsh)

Flags:
  -h, --help    显示帮助信息
```

---

## 💡 性能与优化建议

1. **开启 BBR 拥塞控制算法**：  
   建议在服务器上开启 BBR 加速以获得更优秀的吞吐与更低延迟：
   ```bash
   echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
   echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
   sysctl -p
   ```
2. **防火墙与安全组**：  
   请确保云服务器控制台安全组已开放 TCP `80`（证书申请）、`443`（或自定义 Trojan 端口）以及 `8085`（Web 控制台端口）。

---

## 📄 开源协议与致谢

- 本项目基于 [GPL-3.0 License](LICENSE) 开源。
- 核心参考与衍生自 [Jrohy/trojan](https://github.com/Jrohy/trojan)，感谢原作者的杰出贡献。
- 感谢开源社区各类优质组件的支持与启发。
