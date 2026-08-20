#!/bin/bash
# ==============================================================================
# Trojan 多用户管理平台 - 一键极速平滑更新脚本
# 基于开源项目 Jrohy/trojan 深度重构增强
# GitHub: https://github.com/Level6me/trojan
# ==============================================================================

set -e

repo_owner="Level6me"
repo_name="trojan"
repo_branch="main"

download_url="https://github.com/${repo_owner}/${repo_name}/releases/download"
version_check="https://api.github.com/repos/${repo_owner}/${repo_name}/releases/latest"

# 颜色定义
red="\033[31m"
green="\033[32m"
yellow="\033[33m"
blue="\033[36m"
bold_green="\033[1;32m"
bold_blue="\033[1;36m"
reset="\033[0m"

echo -e "${bold_blue}==================================================${reset}"
echo -e "${bold_green}      Trojan 多用户管理平台 - 一键更新程序${reset}"
echo -e "${bold_blue}==================================================${reset}"

# 1. Root 权限检查
if [ "$(id -u)" != "0" ]; then
    echo -e "${red}❌ 错误: 请使用 root 权限运行此更新脚本！${reset}"
    exit 1
fi

# 2. 机器架构检查
arch=$(uname -m 2> /dev/null)
if [[ $arch == "x86_64" ]]; then
    bin_name="trojan-linux-amd64"
elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
    bin_name="trojan-linux-arm64"
else
    echo -e "${yellow}❌ 暂不支持当前系统架构: $arch${reset}"
    exit 1
fi

# 3. 检查当前是否已安装
if [ ! -f /usr/local/bin/trojan ]; then
    echo -e "${yellow}⚠️ 未在 /usr/local/bin/trojan 发现现有程序，将执行初次安装流程...${reset}"
    curl -sSL "https://raw.githubusercontent.com/${repo_owner}/${repo_name}/${repo_branch}/install.sh" | bash
    exit 0
fi

# 4. 获取最新版本号
latest_version=$(curl -H 'Cache-Control: no-cache' -s "$version_check" | grep 'tag_name' | cut -d\" -f4 2>/dev/null || true)
if [[ -z "$latest_version" ]]; then
    latest_version="v2.8.9"
fi

echo -e "${blue}📦 目标版本: ${latest_version} (${bin_name})${reset}"
echo -e "${blue}⏳ 正在下载最新核心可执行文件...${reset}"

tmp_file="/tmp/trojan-update-bin-$$"
download_succ=0

# 主下载源 (GitHub Release)
curl -L --fail --progress-bar "$download_url/$latest_version/$bin_name" -o "$tmp_file" && download_succ=1

# 备用下载源 (GitHub Raw / jsDelivr CDN)
if [[ $download_succ -eq 0 || ! -s "$tmp_file" ]]; then
    echo -e "${yellow}⚠️ 正在切换至备用加速镜像通道下载...${reset}"
    curl -L --fail --progress-bar "https://raw.githubusercontent.com/${repo_owner}/${repo_name}/${repo_branch}/$bin_name" -o "$tmp_file" || \
    curl -L --fail --progress-bar "https://fastly.jsdelivr.net/gh/${repo_owner}/${repo_name}@${repo_branch}/${bin_name}" -o "$tmp_file"
fi

if [ ! -s "$tmp_file" ]; then
    echo -e "${red}❌ 二进制下载失败，请检查网络后重试。${reset}"
    rm -f "$tmp_file"
    exit 1
fi

chmod +x "$tmp_file"

# 5. 备份旧版本
echo -e "${blue}💾 正在备份当前运行版本至 /usr/local/bin/trojan.bak ...${reset}"
cp -p /usr/local/bin/trojan /usr/local/bin/trojan.bak

# 6. 替换程序
echo -e "${blue}🔄 正在替换最新二进制文件...${reset}"
mv "$tmp_file" /usr/local/bin/trojan
chmod +x /usr/local/bin/trojan

# 7. 升级数据表结构与配置
echo -e "${blue}🛠️ 正在自动检测并升级数据库表结构（限速/历史IP/到期时间/速率统计字段）...${reset}"
/usr/local/bin/trojan upgrade db >/dev/null 2>&1 || true

# 兼容升级旧版数据库字段声明
if [[ -f /usr/local/etc/trojan/config.json && `cat /usr/local/etc/trojan/config.json | grep -w "\"db\""` ]]; then
    sed -i "s/\"db\"/\"database\"/g" /usr/local/etc/trojan/config.json
    systemctl restart trojan >/dev/null 2>&1 || true
fi

# 8. 重启管理服务
echo -e "${blue}🚀 正在平滑重启 Trojan Web 管理服务...${reset}"
systemctl restart trojan-web

sleep 2

# 9. 状态检查
if systemctl is-active --quiet trojan-web; then
    echo -e "${bold_green}==================================================${reset}"
    echo -e "${bold_green}✅ Trojan 管理程序已成功升级至最新版本！${reset}"
    echo -e "${bold_green}==================================================${reset}"
    echo -e "💡 终端直接输入 ${bold_blue}trojan${reset} 即可使用命令行管理。"
    echo -e "🌐 Web 管理面板服务正常运行中 (监听端口: 8085)。"
else
    echo -e "${yellow}⚠️ 服务启动可能需要片刻，请运行 'systemctl status trojan-web' 查看实时状态。${reset}"
fi
