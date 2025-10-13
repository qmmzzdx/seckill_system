#!/bin/bash

# 部署执行日志
LOG_FILE="./seckill_system_check.log"
# 配置文件目录
CONF_DIR="$(pwd)/conf"
# 脚本目录
SCRIPTS_DIR="$(pwd)/scripts"

# 创建配置目录
mkdir -p "$CONF_DIR"

# 检查是否为root用户
function check_root()
{
    if [[ "$EUID" -ne 0 ]]
    then
        echo "请使用root权限运行此脚本" >&2
        return 1
    fi
    return 0
}

# 检查操作系统
function check_os()
{
    # 当前只支持debian
    if [[ -f "/etc/debian_version" ]]
    then
        OS=$(lsb_release -ds 2>/dev/null || cat /etc/*release 2>/dev/null | head -n1 || uname -om)
        echo "检测到操作系统: $OS"
        return 0
    else
        echo "错误: 当前操作系统暂不支持一键部署" >&2
        return 1
    fi
}

# 更新系统包
function update_system_pkg()
{
    echo "更新系统包..."
    apt-get update || {
        echo "错误: 系统包更新失败" >&2
        return 1
    }
    echo "系统包更新成功"
    return 0
}

# 安装必要的工具
function install_tools()
{
    echo "安装必要工具..."

    # 检查工具是否已安装
    local tools=("wget" "curl" "vim" "git" "tar" "gzip")
    local missing_tools=()

    for tool in "${tools[@]}"
    do
        if ! command -v "$tool" &> /dev/null
        then
            missing_tools+=("$tool")
        fi
    done

    if [ ${#missing_tools[@]} -eq 0 ]
    then
        echo "所有必要工具已安装，跳过"
        return 0
    fi
    echo "需要安装的工具: ${missing_tools[*]}"

    apt-get install -y "${missing_tools[@]}" || {
        echo "错误: 工具安装失败" >&2
        return 1
    }
    echo "工具安装完成"
    return 0
}

# 安装Go环境
function install_go()
{
    echo "检查Go环境..."

    if command -v go &> /dev/null
    then
        GO_VERSION=$(go version | awk '{print $3}')
        echo "Go已安装: $GO_VERSION"

        # 检查环境变量是否已设置
        if grep -q "/usr/local/go/bin" /etc/profile
        then
            echo "Go环境变量已设置，跳过"
            return 0
        else
            echo "Go环境变量未设置，添加环境变量..."
            echo "export PATH=\$PATH:/usr/local/go/bin" >> "/etc/profile"
            echo "export GOPATH=\$HOME/go" >> "/etc/profile"
            echo "export PATH=\$PATH:\$GOPATH/bin" >> "/etc/profile"
            source "/etc/profile" || {
                echo "警告: 加载环境变量失败" >&2
            }
            echo "Go环境变量已设置"
            return 0
        fi
    fi

    echo "安装Go..."
    GO_VERSION="1.24.2"
    if [[ ! -f "go${GO_VERSION}.linux-amd64.tar.gz" ]]
    then
        wget -q "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz" || {
            echo "错误: 下载Go失败" >&2
            return 1
        }
    else
        echo "Go安装包已存在，跳过下载"
    fi

    # 检查Go是否已安装
    if [[ ! -d "/usr/local/go" ]]
    then
        tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz" || {
            echo "错误: 解压Go失败" >&2
            return 1
        }
    else
        echo "Go已安装，跳过解压"
    fi

    # 设置环境变量
    echo "export PATH=\$PATH:/usr/local/go/bin" >> "/etc/profile"
    echo "export GOPATH=\$HOME/go" >> "/etc/profile"
    echo "export PATH=\$PATH:\$GOPATH/bin" >> "/etc/profile"

    # 使环境变量生效
    source "/etc/profile" || {
        echo "警告: 加载环境变量失败" >&2
    }
    
    # 清理
    rm -f "go${GO_VERSION}.linux-amd64.tar.gz" || {
        echo "警告: 清理Go安装包失败" >&2
    }
    echo "Go安装完成"
    return 0
}

# 编译和运行应用
function build_and_run_app()
{
    echo "编译和运行秒杀系统应用..."

    # 确保在项目目录中
    if [[ ! -f "./go.mod" ]]
    then
        echo "错误: 未找到go.mod文件，请确保在项目根目录中运行此脚本" >&2
        return 1
    fi

    # 下载依赖
    echo "下载Go依赖..."
    go mod download || {
        echo "错误: 下载Go依赖失败" >&2
        return 1
    }

    # 编译应用
    echo "编译应用..."
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o seckill_system cmd/gateway/main.go || {
        echo "错误: 编译应用失败" >&2
        return 1
    }

    # 更新权限
    if ! chmod 755 "./seckill_system"
    then
        echo "错误: 更新权限失败" >&2
        return 1
    fi
    echo "创建秒杀应用文件成功"

    # 检查应用是否已经在运行
    if pgrep -f "seckill_system" > /dev/null
    then
        echo "检测到秒杀系统已在运行，先停止现有进程..."
        pkill -f "seckill_system"
        sleep 2
    fi

    # 后台运行应用并记录PID
    echo "在后台运行秒杀系统应用..."
    nohup ./seckill_system &
    APP_PID=$!
    
    # 输出应用PID
    echo "seckill_system PID: $APP_PID"
    
    # 等待应用启动
    echo "等待应用启动..."
    sleep 5
    
    # 检查应用是否成功启动
    if ps -p $APP_PID > /dev/null
    then
        echo "秒杀系统应用已成功在后台运行 (PID: $APP_PID)"
        # 检查服务是否可访问
        echo "检查服务状态..."
        for i in {1..10}; do
            if curl -s http://localhost:8000/api/goods/1001 > /dev/null 2>&1
            then
                echo "服务已就绪，可以访问 http://localhost:8000"
                return 0
            fi
            echo "等待服务启动... ($i/10)"
            sleep 2
        done
    else
        echo "错误: 应用启动失败，请检查日志文件" >&2
        return 1
    fi
    return 0
}

# 秒杀系统部署主函数
function main()
{
    echo "开始部署秒杀系统..."

    # 检测是否为root用户
    if ! check_root
    then
        return 1
    fi

    # 检测是否是支持的操作系统
    if ! check_os
    then
        return 1
    fi

    # 更新系统包
    if ! update_system_pkg
    then
        return 1
    fi

    # 安装必要工具
    if ! install_tools
    then
        return 1
    fi

    # 安装Mysql数据库
    if ! bash "${SCRIPTS_DIR}/mysql/install_mysql.sh"
    then
        return 1
    fi

    # 安装Redis
    if ! bash "${SCRIPTS_DIR}/redis/install_redis.sh"
    then
        return 1
    fi

    # 安装Kafka
    if ! bash "${SCRIPTS_DIR}/kafka/install_kafka.sh"
    then
        return 1
    fi

    # 安装Etcd
    if ! bash "${SCRIPTS_DIR}/etcd/install_etcd.sh"
    then
        return 1
    fi

    # 安装Go环境
    if ! install_go
    then
        return 1
    fi

    # 编译和运行应用
    if ! build_and_run_app
    then
        return 1
    fi

    echo "秒杀系统部署完成!"
    echo "服务访问地址:"
    echo "- 秒杀系统: http://localhost:8000"
    return 0
}

set -x
main > "${LOG_FILE}" 2>&1
exit $?
