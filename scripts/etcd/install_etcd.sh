#!/bin/bash

# 配置文件目录
CONF_DIR="$(pwd)/conf"

# 安装Etcd
function install_etcd()
{
    echo "检查Etcd安装状态..."

    # 检查Etcd是否已安装
    if command -v etcd &> /dev/null && systemctl is-active --quiet etcd
    then
        echo "Etcd已安装并运行，跳过安装"
        return 0
    fi
    echo "安装Etcd..."

    # 下载Etcd
    if [[ ! -f "etcd-v3.6.4-linux-amd64.tar.gz" ]]
    then
        wget -q "https://github.com/etcd-io/etcd/releases/download/v3.6.4/etcd-v3.6.4-linux-amd64.tar.gz" || {
            echo "错误: 下载Etcd失败" >&2
            return 1
        }
    else
        echo "Etcd安装包已存在，跳过下载"
    fi

    # 解压
    if [[ ! -d "etcd-v3.6.4-linux-amd64" ]]
    then
        tar -xzf "etcd-v3.6.4-linux-amd64.tar.gz" || {
            echo "错误: 解压Etcd失败" >&2
            return 1
        }
    else
        echo "Etcd已解压，跳过解压步骤"
    fi

    # 检查二进制文件是否已存在
    if [[ ! -f "/usr/local/bin/etcd" ]] || [[ ! -f "/usr/local/bin/etcdctl" ]]
    then
        mv "etcd-v3.6.4-linux-amd64/etcd"* /usr/local/bin/ || {
            echo "错误: 移动Etcd二进制文件失败" >&2
            return 1
        }
    else
        echo "Etcd二进制文件已存在，跳过移动"
    fi

    # 创建Etcd数据目录
    mkdir -p /var/lib/etcd

    # 检查服务文件是否已存在
    if [[ ! -f "/etc/systemd/system/etcd.service" ]]
    then
        # 替换服务文件中的CONF_DIR变量
        sed "s|{CONF_DIR}|$CONF_DIR/etcd|g" "$CONF_DIR/etcd/etcd.service" > "/etc/systemd/system/etcd.service" || {
            echo "错误: 创建Etcd服务文件失败" >&2
            return 1
        }
        systemctl daemon-reload || {
            echo "错误: 重新加载systemd配置失败" >&2
            return 1
        }
    else
        echo "Etcd服务文件已存在，跳过创建"
    fi

    # 启动Etcd服务
    systemctl start etcd || {
        echo "错误: 启动Etcd服务失败" >&2
        return 1
    }
    systemctl enable etcd || {
        echo "错误: 设置Etcd服务开机自启失败" >&2
        return 1
    }
    # 清理
    rm -rf "etcd-v3.6.4-linux-amd64"* || {
        echo "警告: 清理Etcd临时文件失败" >&2
    }
    echo "Etcd安装完成"
    return 0
}

install_etcd
exit $?
