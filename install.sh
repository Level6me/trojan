#!/bin/bash
# ==============================================================================
# Trojan 多用户管理平台 - 一键部署与管理脚本
# 基于开源项目 Jrohy/trojan 深度重构增强
# GitHub: https://github.com/Level6me/trojan
# ==============================================================================

# 定义操作变量: 0为否, 1为是
help=0
remove=0
update=0

repo_owner="Level6me"
repo_name="trojan"
repo_branch="main"

download_url="https://github.com/${repo_owner}/${repo_name}/releases/download"
version_check="https://api.github.com/repos/${repo_owner}/${repo_name}/releases/latest"
service_url="https://raw.githubusercontent.com/${repo_owner}/${repo_name}/${repo_branch}/asset/trojan-web.service"

[[ -e /var/lib/trojan-manager ]] && update=1

# Centos 临时取消别名
[[ -f /etc/redhat-release && -z $(echo $SHELL | grep zsh) ]] && unalias -a

[[ -z $(echo $SHELL | grep zsh) ]] && shell_way="bash" || shell_way="zsh"

####### 颜色代码 #######
red="31m"
green="32m"
yellow="33m"
blue="36m"
fuchsia="35m"
bold_green="1;32m"
bold_blue="1;36m"

colorEcho() {
    color=$1
    echo -e "\033[${color}${@:2}\033[0m"
}

####### 参数解析 #######
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        --remove|-u|--uninstall)
            remove=1
            ;;
        --update)
            update=1
            ;;
        -h|--help)
            help=1
            ;;
        *)
            ;;
    esac
    shift
done

help() {
    echo "使用方式: bash $0 [-h|--help] [--update] [--remove]"
    echo "  -h, --help           显示此帮助信息"
    echo "      --update         更新 Trojan 管理服务程序"
    echo "      --remove         彻底卸载 Trojan 核心及管理程序"
    return 0
}

removeTrojan() {
    colorEcho $yellow "⚠️ 正在开始彻底卸载 Trojan 及其管理服务..."
    
    # 停止服务
    systemctl stop trojan-web >/dev/null 2>&1
    systemctl stop trojan >/dev/null 2>&1
    systemctl stop trojan-go >/dev/null 2>&1
    systemctl disable trojan-web >/dev/null 2>&1
    systemctl disable trojan >/dev/null 2>&1

    # 移除 trojan 核心
    rm -rf /usr/bin/trojan >/dev/null 2>&1
    rm -rf /usr/local/etc/trojan >/dev/null 2>&1
    rm -f /etc/systemd/system/trojan.service >/dev/null 2>&1
    rm -f /etc/systemd/system/trojan-go.service >/dev/null 2>&1

    # 移除 trojan 管理程序
    rm -f /usr/local/bin/trojan >/dev/null 2>&1
    rm -f /usr/local/bin/trojan.bak* >/dev/null 2>&1
    rm -rf /var/lib/trojan-manager >/dev/null 2>&1
    rm -f /etc/systemd/system/trojan-web.service >/dev/null 2>&1

    systemctl daemon-reload

    # 移除 trojan 专用容器
    docker rm -f trojan-mysql trojan-mariadb >/dev/null 2>&1
    rm -rf /home/mysql /home/mariadb >/dev/null 2>&1
    
    # 清理环境变量
    sed -i '/trojan/d' ~/.rc 2>/dev/null
    
    colorEcho $green "✅ Trojan 及管理程序已彻底卸载完成！"
}

checkSys() {
    # 检查是否为 Root
    if [ $(id -u) != "0" ]; then
        colorEcho $red "❌ 错误: 请以 root 用户身份运行此安装脚本！"
        exit 1
    fi

    arch=$(uname -m 2> /dev/null)
    if [[ $arch != "x86_64" && $arch != "aarch64" && $arch != "arm64" ]]; then
        colorEcho $yellow "❌ 暂不支持当前架构: $arch (仅支持 x86_64 / arm64)"
        exit 1
    fi

    if [[ `command -v apt-get` ]]; then
        package_manager='apt-get'
    elif [[ `command -v dnf` ]]; then
        package_manager='dnf'
    elif [[ `command -v yum` ]]; then
        package_manager='yum'
    else
        colorEcho $red "❌ 未知系统包管理器，仅支持 Debian/Ubuntu/CentOS/Fedora/RHEL 系列系统！"
        exit 1
    fi

    # 缺失 /usr/local/bin 路径时自动补全 PATH
    if [[ -z $(echo $PATH | grep "/usr/local/bin") ]]; then
        echo 'export PATH=$PATH:/usr/local/bin' >> /etc/bashrc 2>/dev/null
        echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile 2>/dev/null
        export PATH=$PATH:/usr/local/bin
    fi
}

# 安装依赖项
installDependent() {
    colorEcho $blue "📦 正在安装运行依赖组件..."
    if [[ ${package_manager} == 'dnf' || ${package_manager} == 'yum' ]]; then
        ${package_manager} install -y curl wget socat crontabs bash-completion tar xz
    else
        ${package_manager} update -y
        ${package_manager} install -y curl wget socat cron bash-completion tar xz-utils
    fi
}

setupCron() {
    if [[ `crontab -l 2>/dev/null | grep acme` ]]; then
        if [[ -z `crontab -l 2>/dev/null | grep trojan-web` || `crontab -l 2>/dev/null | grep trojan-web | grep "&"` ]]; then
            origin_time_zone=$(date -R | awk '{printf"%d",$6}')
            local_time_zone=${origin_time_zone%00}
            beijing_zone=8
            beijing_update_time=3
            diff_zone=$[$beijing_zone-$local_time_zone]
            local_time=$[$beijing_update_time-$diff_zone]
            if [ $local_time -lt 0 ]; then
                local_time=$[$24+$local_time]
            elif [ $local_time -ge 24 ]; then
                local_time=$[$local_time-24]
            fi
            crontab -l 2>/dev/null | sed '/acme.sh/d' > /tmp/crontab.txt
            echo "0 ${local_time}"' * * * systemctl stop trojan-web; "/root/.acme.sh"/acme.sh --cron --home "/root/.acme.sh" > /dev/null; systemctl start trojan-web' >> /tmp/crontab.txt
            crontab /tmp/crontab.txt
            rm -f /tmp/crontab.txt
        fi
    fi
}

installTrojan() {
    local show_tip=0
    if [[ $update == 1 ]]; then
        systemctl stop trojan-web >/dev/null 2>&1
        [[ -f /usr/local/bin/trojan ]] && cp -p /usr/local/bin/trojan /usr/local/bin/trojan.bak 2>/dev/null
        rm -f /usr/local/bin/trojan
    fi

    # 获取最新发布版本号
    latest_version=$(curl -H 'Cache-Control: no-cache' -s "$version_check" | grep 'tag_name' | cut -d\" -f4)
    if [[ -z "$latest_version" ]]; then
        latest_version="v2.8.9"
    fi

    colorEcho $blue "⬇️ 正在获取 Trojan 管理程序 [$latest_version]..."

    if [[ $arch == "x86_64" ]]; then
        bin_name="trojan-linux-amd64"
    else
        bin_name="trojan-linux-arm64"
    fi

    # 下载二进制
    download_succ=0
    curl -L --fail --progress-bar "$download_url/$latest_version/$bin_name" -o /usr/local/bin/trojan && download_succ=1
    
    # 备用源（若 Release 未发布或网络受限，直接从加速通道拉取）
    if [[ $download_succ -eq 0 || ! -s /usr/local/bin/trojan ]]; then
        colorEcho $yellow "⚠️ 正在从备用镜像拉取最新管理程序..."
        curl -L --fail --progress-bar "https://raw.githubusercontent.com/${repo_owner}/${repo_name}/${repo_branch}/$bin_name" -o /usr/local/bin/trojan || \
        curl -L --fail --progress-bar "https://fastly.jsdelivr.net/gh/${repo_owner}/${repo_name}@${repo_branch}/${bin_name}" -o /usr/local/bin/trojan
    fi

    chmod +x /usr/local/bin/trojan

    # 配置 systemd 服务文件
    if [[ ! -e /etc/systemd/system/trojan-web.service ]]; then
        show_tip=1
        cat > /etc/systemd/system/trojan-web.service << 'UNIT_EOF'
[Unit]
Description=trojan-web
Documentation=https://github.com/Level6me/trojan
After=network.target network-online.target nss-lookup.target mysql.service mariadb.service mysqld.service docker.service

[Service]
Type=simple
StandardError=journal
ExecStart=/usr/local/bin/trojan web -p 8085
ExecReload=/bin/kill -HUP 
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
UNIT_EOF
        systemctl daemon-reload
        systemctl enable trojan-web >/dev/null 2>&1
    fi

    # 命令补全环境变量
    if [[ -z $(grep "trojan completion" ~/.rc 2>/dev/null) ]]; then
        echo "source <(trojan completion )" >> ~/.rc 2>/dev/null
    fi
    source ~/.rc 2>/dev/null

    if [[ $update == 0 ]]; then
        colorEcho $bold_green "🎉 Trojan 管理程序安装成功！"
        echo -e "💡 终端中输入命令 \033[1;36mtrojan\033[0m 即可唤出交互式管理菜单。\n"
        /usr/local/bin/trojan
    else
        # 兼容旧配置与数据表自动升级
        if [[ -f /usr/local/etc/trojan/config.json && `cat /usr/local/etc/trojan/config.json | grep -w "\"db\""` ]]; then
            sed -i "s/\"db\"/\"database\"/g" /usr/local/etc/trojan/config.json
            systemctl restart trojan >/dev/null 2>&1
        fi
        /usr/local/bin/trojan upgrade db >/dev/null 2>&1
        if [[ -f /usr/local/etc/trojan/config.json && -z `cat /usr/local/etc/trojan/config.json | grep sni` ]]; then
            /usr/local/bin/trojan upgrade config >/dev/null 2>&1
        fi
        systemctl restart trojan-web >/dev/null 2>&1
        colorEcho $bold_green "🎉 Trojan 管理程序已成功平滑更新至最新版本！"
    fi

    setupCron

    if [[ $show_tip == 1 ]]; then
        echo -e "\n🌐 浏览器访问 \033[1;36mhttps://您的域名:8085\033[0m 或 \033[1;36mhttps://您的域名\033[0m 即可登录现代化 Web 控制台管理用户。"
    fi
}

main() {
    [[ ${help} == 1 ]] && help && return
    [[ ${remove} == 1 ]] && removeTrojan && return
    
    echo -e "\033[1;36m==================================================\033[0m"
    echo -e "\033[1;32m      Trojan 多用户管理平台一键部署程序\033[0m"
    echo -e "\033[1;30m      基于原项目 Jrohy/trojan 深度重构增强\033[0m"
    echo -e "\033[1;36m==================================================\033[0m"

    [[ $update == 0 ]] && colorEcho $blue "🚀 正在检测环境并准备安装 Trojan 管理程序..." || colorEcho $blue "🔄 正在准备升级 Trojan 管理程序..."
    checkSys
    [[ $update == 0 ]] && installDependent
    installTrojan
}

main
