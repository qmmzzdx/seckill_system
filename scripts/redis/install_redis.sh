#!/bin/bash

# 配置文件目录
CONF_DIR="$(pwd)/conf"

# 安装Redis
function install_redis()
{
    echo "检查Redis安装状态..."

    # 创建Redis配置目录
    mkdir -p "$CONF_DIR/redis/cluster"
    mkdir -p "$CONF_DIR/redis/services"  # 存放服务文件的目录

    # 检查Redis是否已安装
    if command -v redis-server &> /dev/null
    then
        echo "Redis已安装，检查集群状态..."
        
        # 检查Redis集群是否已创建
        if redis-cli -p 7000 cluster nodes 2>/dev/null | grep -q "master"
        then
            echo "Redis集群已存在，跳过Redis安装"
            return 0
        else
            echo "Redis集群未创建，继续配置..."
        fi
    else
        echo "安装Redis..."
        apt-get install -y redis-server || {
            echo "错误: Redis安装失败" >&2
            return 1
        }
    fi

    # 创建6个Redis实例的配置文件
    redis_ports=(7000 7001 7002 7003 7004 7005)
    for port in "${redis_ports[@]}"
    do
        # 创建redis端口实例文件夹
        mkdir -p "/var/lib/redis/$port"
        chown redis:redis "/var/lib/redis/$port"

        # 创建日志目录
        mkdir -p /var/log/redis
        chown redis:redis /var/log/redis

        # 创建运行目录
        mkdir -p /var/run/redis
        chown redis:redis /var/run/redis

        # 检查Redis实例服务文件是否已存在
        if [[ ! -f "/etc/systemd/system/redis-$port.service" ]]
        then
            # 使用sed替换模板中的占位符，生成服务文件
            echo "生成Redis服务文件: redis-$port.service"
            sed -e "s/{PORT}/$port/g" -e "s|{CONF_DIR}|$CONF_DIR|g" "$CONF_DIR/redis/redis_service_template.ini" > "$CONF_DIR/redis/services/redis-$port.service" || {
                echo "错误: 生成Redis实例 $port 服务文件失败" >&2
                return 1
            }
            # 复制服务文件到系统目录
            cp "$CONF_DIR/redis/services/redis-$port.service" "/etc/systemd/system/redis-$port.service" || {
                echo "错误: 复制Redis实例 $port 服务文件失败" >&2
                return 1
            }
            systemctl daemon-reload || {
                echo "错误: 重新加载systemd配置失败" >&2
                return 1
            }
        fi

        # 检查Redis实例是否已启动
        if ! systemctl is-active --quiet "redis-$port"
        then
            # 启动Redis实例
            systemctl start "redis-$port" || {
                echo "错误: 启动Redis实例 $port 失败" >&2
                # 检查错误日志
                if [[ -f "/var/log/redis/redis-server-$port.log" ]]
                then
                    echo "Redis实例 $port 错误日志:"
                    tail -n 20 "/var/log/redis/redis-server-$port.log"
                fi
                return 1
            }
            systemctl enable "redis-$port" || {
                echo "错误: 设置Redis实例 $port 开机自启失败" >&2
                return 1
            }
            echo "Redis实例 $port 已启动"
        else
            echo "Redis实例 $port 已运行，跳过"
        fi
    done

    echo "Redis实例安装完成，正在初始化集群..."
    # 等待所有Redis实例启动
    sleep 10

    # 检查所有Redis实例是否正常运行
    for port in "${redis_ports[@]}"
    do
        if ! redis-cli -p "$port" ping &> /dev/null
        then
            echo "错误: Redis实例 $port 未正常运行" >&2
            return 1
        fi
    done

    # 检查Redis集群是否已创建
    if ! redis-cli -p 7000 cluster nodes 2>/dev/null | grep -q "master"
    then
        # 创建Redis集群
        echo "创建Redis集群..."
        echo "yes" | redis-cli --cluster create \
            127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 \
            127.0.0.1:7003 127.0.0.1:7004 127.0.0.1:7005 \
            --cluster-replicas 1 || {
            echo "错误: Redis集群创建失败" >&2
            echo "尝试手动创建集群，请运行:"
            echo "redis-cli --cluster create 127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 127.0.0.1:7003 127.0.0.1:7004 127.0.0.1:7005 --cluster-replicas 1"
            return 1
        }
        echo "Redis集群创建完成"
    else
        echo "Redis集群已存在，跳过集群创建"
    fi

    # 验证集群状态
    echo "验证Redis集群状态..."
    if redis-cli -p 7000 cluster info
    then
        echo "Redis集群状态正常"
    else
        echo "警告: 无法获取Redis集群信息" >&2
    fi
    return 0
}

install_redis
exit $?
