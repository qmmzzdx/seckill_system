#!/bin/bash

# 安装MySQL
function install_mysql()
{
    echo "检查MySQL安装状态..."
    
    # 定义MySQL服务名
    MYSQL_SERVICE="mysql"
    if ! systemctl list-unit-files | grep -q "^mysql.service"
    then
        if systemctl list-unit-files | grep -q "^mysqld.service"
        then
            MYSQL_SERVICE="mysqld"
        fi
    fi

    # 检查MySQL是否已安装
    if command -v mysql &> /dev/null && systemctl is-active --quiet "$MYSQL_SERVICE"
    then
        echo "MySQL已安装并运行，检查数据库..."
        
        # 检查数据库是否存在
        if mysql -uroot -p"123456" -e "USE seckill_db;" &> /dev/null
        then
            echo "数据库seckill_db已存在，跳过MySQL安装"
            return 0
        else
            echo "数据库seckill_db不存在，创建数据库..."
            mysql -uroot -p"123456" -e "CREATE DATABASE IF NOT EXISTS seckill_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" || {
                echo "错误: 创建数据库失败" >&2
                return 1
            }
            # 验证数据库是否创建成功
            if mysql -uroot -p"123456" -e "USE seckill_db; SELECT 1;" &> /dev/null
            then
                echo "数据库创建并验证成功"
            else
                echo "错误: 数据库创建后验证失败" >&2
                return 1
            fi
            return 0
        fi
    fi

    # 更新包列表
    echo "更新包列表..."
    apt-get update || {
        echo "错误: 更新包列表失败" >&2
        return 1
    }

    # 安装MySQL服务器
    echo "安装MySQL服务器..."
    apt-get install -y mysql-server-8.0 || {
        echo "错误: 安装MySQL服务器失败" >&2
        return 1
    }
    
    # 启动MySQL服务
    echo "启动MySQL服务..."
    systemctl start "$MYSQL_SERVICE" || {
        echo "错误: 启动MySQL服务失败" >&2
        return 1
    }
    
    echo "设置MySQL服务开机自启..."
    systemctl enable "$MYSQL_SERVICE" || {
        echo "错误: 设置MySQL服务开机自启失败" >&2
        return 1
    }
    
    # 等待MySQL服务完全启动
    echo "等待MySQL服务启动..."
    sleep 10  # 增加等待时间，确保服务完全启动

    # 检查MySQL服务状态
    if ! systemctl is-active --quiet "$MYSQL_SERVICE"
    then
        echo "错误: MySQL服务未正常运行" >&2
        return 1
    fi

    # 设置root密码
    echo "设置MySQL root密码..."
    # 尝试多个可能的日志文件位置
    TEMP_PASSWORD=""
    for log_file in "/var/log/mysql/error.log" "/var/log/mysqld.log" "/var/log/mysql.log"
    do
        if [[ -f "$log_file" ]]
        then
            TEMP_PASSWORD=$(grep 'temporary password' "$log_file" 2>/dev/null | awk '{print $NF}')
            if [[ -n "$TEMP_PASSWORD" ]]
            then
                echo "在 $log_file 中找到临时密码"
                break
            fi
        fi
    done

    if [[ -n "$TEMP_PASSWORD" ]]
    then
        echo "使用临时密码设置root密码..."
        mysql -u root -p"$TEMP_PASSWORD" --connect-expired-password -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '123456';" || {
            echo "错误: 使用临时密码设置root密码失败" >&2
            return 1
        }
    else
        echo "尝试直接设置root密码..."
        # 检查是否可以无密码登录
        if mysql -u root -e "SELECT 1;" &> /dev/null
        then
            mysql -u root -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '123456';" || {
                echo "错误: 设置root密码失败" >&2
                return 1
            }
        else
            # 尝试用当前密码登录
            if mysql -u root -p"123456" -e "SELECT 1;" &> /dev/null
            then
                echo "root密码已设置，跳过"
            else
                echo "错误: 无法设置MySQL root密码" >&2
                return 1
            fi
        fi
    fi

    # 验证密码是否设置成功
    if ! mysql -u root -p"123456" -e "SELECT 1;" &> /dev/null
    then
        echo "错误: MySQL root密码设置后验证失败" >&2
        return 1
    fi

    mysql -uroot -p"123456" -e "FLUSH PRIVILEGES;" || {
        echo "错误: 刷新权限失败" >&2
        return 1
    }

    # 创建数据库
    echo "创建seckill_db数据库..."
    mysql -uroot -p"123456" -e "CREATE DATABASE IF NOT EXISTS seckill_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" || {
        echo "错误: 创建数据库失败" >&2
        return 1
    }

    # 验证数据库是否创建成功
    if mysql -uroot -p"123456" -e "USE seckill_db; SELECT 1;" &> /dev/null
    then
        echo "数据库创建并验证成功"
    else
        echo "错误: 数据库创建后验证失败" >&2
        return 1
    fi
    echo "MySQL安装完成"
    return 0
}

install_mysql
exit $?
