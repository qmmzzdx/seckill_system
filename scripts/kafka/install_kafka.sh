#!/bin/bash

# 配置文件目录
CONF_DIR="$(pwd)/conf"

# 安装Kafka (Kraft模式)
function install_kafka()
{
    echo "检查Kafka安装状态..."

    # 检查Kafka是否已安装（检查安装目录、数据目录、服务文件）
    local KAFKA_INSTALLED="true"

    # 检查Kafka安装目录和关键文件
    if [[ ! -d "/opt/kafka" ]] || [[ ! -f "/opt/kafka/bin/kafka-server-start.sh" ]]
    then
        KAFKA_INSTALLED=false
        echo "Kafka安装目录或关键文件未找到"
    fi

    # 检查数据目录
    for i in {1..3}
    do
        if [[ ! -d "/var/lib/kafka${i}" ]]
        then
            KAFKA_INSTALLED=false
            echo "Kafka数据目录 /var/lib/kafka${i} 未找到"
        fi
    done

    # 检查服务文件
    for i in {1..3}
    do
        if [[ ! -f "/etc/systemd/system/kafka${i}.service" ]]
        then
            KAFKA_INSTALLED=false
            echo "Kafka服务文件 kafka${i}.service 未找到"
        fi
    done

    # 检查服务是否正在运行（至少有一个节点在运行）
    local SERVICE_RUNNING=false
    for i in {1..3}
    do
        if systemctl is-active --quiet "kafka${i}" 2>/dev/null
        then
            SERVICE_RUNNING=true
            break
        fi
    done

    # 如果Kafka已完整安装且至少有一个服务在运行，则跳过安装
    if [[ "$KAFKA_INSTALLED" == "true" && "$SERVICE_RUNNING" == "true" ]]
    then
        echo "Kafka已完整安装并正在运行，跳过安装"
        echo "节点状态:"
        for i in {1..3}
        do
            if systemctl is-active --quiet "kafka${i}" 2>/dev/null
            then
                echo "  kafka${i}: 运行中"
            else
                echo "  kafka${i}: 已停止"
            fi
        done
        return 0
    elif [[ "$KAFKA_INSTALLED" == "true" && "$SERVICE_RUNNING" == "false" ]]
    then
        echo "Kafka已安装但服务未运行，尝试启动服务..."
        for i in {1..3}
        do
            systemctl start "kafka${i}" 2>/dev/null && echo "启动kafka${i}服务" || echo "启动kafka${i}服务失败"
        done
        return 0
    fi

    echo "开始安装Kafka (Kraft模式)..."

    # 检查并安装Java 17环境
    echo "检查Java版本..."
    if ! command -v java &> /dev/null
    then
        JAVA_VERSION=0
    else
        JAVA_VERSION=$(java -version 2>&1 | awk -F '"' '/version/ {print $2}' | awk -F '.' '{print $1}')
    fi
    
    if [[ "$JAVA_VERSION" -lt 17 ]]
    then
        echo "安装Java 17环境..."
        
        # 先更新包列表
        apt-get update
        
        # 安装Java 17
        apt-get install -y openjdk-17-jdk || {
            echo "错误: 安装Java 17失败" >&2
            return 1
        }
        
        # 设置环境变量
        export JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64
        export PATH=$JAVA_HOME/bin:$PATH
        
        # 添加到profile确保永久生效
        echo "export JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64" >> /etc/profile
        echo "export PATH=\$JAVA_HOME/bin:\$PATH" >> /etc/profile
        source /etc/profile
    else
        echo "Java版本满足要求: $(java -version 2>&1 | head -n 1)"
    fi

    # 要下载的kafka文件(使用kraft)
    KAFKA_FILE="kafka_2.13-4.1.0"

    # 下载Kafka
    if [[ ! -f "$KAFKA_FILE.tgz" ]]
    then
        echo "下载Kafka..."
        wget -q "https://downloads.apache.org/kafka/4.1.0/$KAFKA_FILE.tgz" || {
            echo "错误: 下载Kafka失败" >&2
            return 1
        }
    fi

    # 解压Kafka
    echo "解压Kafka..."
    tar -xzf "$KAFKA_FILE.tgz" || {
        echo "错误: 解压Kafka失败" >&2
        return 1
    }

    # 移动到/opt目录
    if [[ ! -d "/opt/kafka" ]]
    then
        mv "$KAFKA_FILE" /opt/kafka || {
            echo "错误: 移动Kafka到/opt目录失败" >&2
            return 1
        }
    fi

    # 创建Kafka数据目录（3个节点）
    for i in {1..3}; do
        mkdir -p "/var/lib/kafka$i"
    done
    mkdir -p /var/log/kafka

    # 创建Kafka用户
    if ! id -u kafka &>/dev/null
    then
        useradd -r -s /bin/false kafka
    fi

    # 配置Kraft模式
    echo "配置Kraft模式..."
    
    # 生成集群ID
    CLUSTER_ID=$(/opt/kafka/bin/kafka-storage.sh random-uuid)

    # 创建kraft配置目录
    mkdir -p /opt/kafka/config/kraft/
    
    # 从配置文件目录复制Kraft配置文件
    echo "复制Kafka配置文件..."
    for i in {1..3}
    do
        cp "$CONF_DIR/kafka/server$i.properties" /opt/kafka/config/kraft/
    done
    
    # 格式化每个节点的存储目录
    echo "格式化Kafka存储目录..."
    for i in {1..3}
    do
        /opt/kafka/bin/kafka-storage.sh format -t $CLUSTER_ID -c "/opt/kafka/config/kraft/server$i.properties"
    done

    # 设置权限
    chown -R kafka:kafka /opt/kafka
    for i in {1..3}
    do
        chown -R kafka:kafka "/var/lib/kafka$i"
    done
    chown -R kafka:kafka /var/log/kafka

    # 复制systemd服务文件
    echo "创建Kafka服务文件..."
    for i in {1..3}
    do
        cp "$CONF_DIR/kafka/kafka$i.service" "/etc/systemd/system/"
    done

    # 重新加载systemd
    systemctl daemon-reload

    # 启动所有Kafka节点
    echo "启动Kafka服务..."
    for i in {1..3}
    do
        echo "启动Kafka节点$i..."
        systemctl start "kafka$i" || {
            echo "错误: 启动Kafka节点$i失败" >&2
            echo "查看详细日志: sudo journalctl -u kafka$i -n 50"
        }
        systemctl enable "kafka$i" || {
            echo "错误: 设置Kafka节点$i开机自启失败" >&2
        }
    done

    # 等待Kafka启动
    echo "等待Kafka服务启动..."
    sleep 20

    # 检查所有节点服务状态
    ALL_RUNNING=true
    for i in {1..3}
    do
        if systemctl is-active --quiet "kafka$i"
        then
            echo "Kafka节点$i 启动成功"
        else
            echo "Kafka节点$i 启动失败"
            ALL_RUNNING=false
        fi
    done

    if [[ "$ALL_RUNNING" == "true" ]]
    then
        echo "Kafka集群安装成功并正在运行 (Kraft模式)"

        # 测试Kafka集群连接
        echo "测试Kafka集群连接..."
        timeout 10s /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092,127.0.0.1:9094,127.0.0.1:9096 --list
        if [[ $? -eq 0 ]]
        then
            echo "Kafka集群连接测试成功"
        else
            echo "警告: Kafka集群连接测试失败，但服务正在运行"
        fi
        
        return 0
    else
        echo "Kafka集群启动失败，查看日志: sudo journalctl -u kafka* -n 50" >&2
        return 1
    fi
}

# 执行安装
install_kafka
exit $?
