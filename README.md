# 秒杀系统 (Seckill System)

## 🎯 项目概述

一个基于Go语言构建的高性能、高可用的分布式秒杀系统。系统采用微服务架构思想，集成了多种中间件和技术栈，有效解决了高并发场景下的超卖、性能瓶颈和数据一致性问题。

### ✨ 核心特性

- 🔒 **分布式锁保护** - 基于Etcd的强一致性分布式锁，彻底解决超卖问题
- ⚡ **高性能处理** - Redis预减库存 + 异步消息队列，支持万级QPS
- 🛡️ **全方位防护** - 多层级限流、黑名单、令牌验证等安全机制
- 🔄 **动态配置** - Etcd配置中心支持实时配置更新
- 🧪 **完整测试** - 包含单元测试、集成测试和压力测试
- 🚀 **一键部署** - 自动化脚本快速部署所有依赖服务

## 🏗️ 系统架构

### 技术架构图

```
客户端请求
    ↓
Gateway网关 (Gin)
    ↓
业务处理层 (Service) → 分布式锁 (Etcd) → 配置中心 (Etcd)
    ↓
数据访问层 (Repository) → 消息队列 (Kafka)
    ↓
存储层 (MySQL + Redis Cluster)
```

### 核心业务流程

#### 1. 秒杀令牌获取流程
```
用户认证 → 检查秒杀开关 → 黑名单检查 → 商品验证 → 
活动时间检查 → 库存检查 → 限流检查 → 生成令牌
```

#### 2. 秒杀下单流程（分布式锁保护）
```
获取分布式锁 → 令牌验证 → Redis预减库存 → 数据库事务 → 
乐观锁扣库存 → 创建订单 → 发送Kafka消息 → 释放分布式锁
```

#### 3. 支付流程
```
支付请求 → 支付处理 → 发送支付消息 → 
异步更新订单状态 → 失败时恢复库存
```

## 🛠️ 技术栈

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| **开发语言** | Go 1.24+ | 高性能并发处理 |
| **Web框架** | Gin | 轻量级HTTP框架 |
| **数据库** | MySQL 8.0+ | 关系型数据存储 |
| **缓存** | Redis Cluster (6节点) | 分布式缓存，库存预减 |
| **消息队列** | Kafka (3节点集群) | 异步消息处理，系统解耦 |
| **配置中心** | Etcd | 动态配置管理，分布式锁 |
| **ORM** | GORM | 数据库操作 |
| **测试** | Go Testing | 单元测试和集成测试 |

## 📁 项目结构

```bash
seckill_system/
├── README.md
├── cmd/
│   └── gateway/
│       └── main.go                 # 网关入口
├── conf/
│   ├── conf.yaml                   # 主配置文件
│   ├── etcd/                       # Etcd服务文件 
│   ├── kafka/                      # Kafka服务文件
│   └── redis/                      # Redis服务文件  
├── config/
│   └── config.go                   # 配置解析
├── global/
│   └── global.go                   # 全局变量和初始化
├── handler/
│   └── seckill.go                  # 秒杀业务处理器
├── model/
│   └── model.go                    # 数据模型
├── repository/
│   ├── etcd_repository.go          # Etcd配置中心 & 分布式锁
│   ├── good_repository.go          # 商品数据访问
│   ├── kafka_repository.go         # Kafka消息处理
│   └── redis_repository.go         # Redis缓存操作
├── scripts/                        # 部署和测试脚本
├── service/
│   └── good_service.go             # 商品业务服务
├── run_services.sh                 # 一键安装编译脚本
├── test/                           # 完整的测试套件
│   ├── interfaces.go               # 测试接口定义
│   ├── mocks.go                    # Mock实现
│   ├── seckill_handler_test.go     # 业务逻辑测试
│   ├── distributed_lock_test.go    # 分布式锁专项测试
│   └── test_helpers.go             # 测试工具函数
└── web/
    ├── controller/
    │   └── controller.go           # HTTP控制器
    ├── middleware/
    │   └── middleware.go           # 中间件
    └── router/
        └── router.go               # 路由配置
```

## 🚀 快速开始

### 环境要求

- **操作系统**: Ubuntu 18.04+
- **Go版本**: 1.24 或更高
- **内存**: 至少 4GB RAM
- **磁盘空间**: 至少 10GB 可用空间

### 一键部署

```bash
# 克隆项目
git clone <repository-url>
cd seckill_system

# 授予执行权限
chmod +x run_services.sh
chmod +x scripts/*.sh
chmod +x scripts/*/*.sh

# 一键部署所有服务（MySQL, Redis, Kafka, Etcd）
./run_services.sh
```

### 分步部署

#### 1. 安装依赖服务

```bash
# 安装MySQL
./scripts/mysql/install_mysql.sh

# 安装Redis集群(6节点)
./scripts/redis/install_redis.sh

# 安装Kafka集群(3节点)  
./scripts/kafka/install_kafka.sh

# 安装Etcd
./scripts/etcd/install_etcd.sh
```

#### 2. 配置应用

编辑 `conf/conf.yaml` 文件，根据实际环境调整配置：

```yaml
server:
  port: 8000

database:
  host: 127.0.0.1
  port: 3306
  user: root
  password: 123456
  name: seckill_db

redis:
  cluster_nodes: 127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002,127.0.0.1:7003,127.0.0.1:7004,127.0.0.1:7005
  password: ""

kafka:
  brokers: 127.0.0.1:9092,127.0.0.1:9094,127.0.0.1:9096
  topic: seckill_orders
  group_id: seckill_group

etcd:
  host: 127.0.0.1:2379
  dial_timeout: 5
  username: ""
  password: ""

log:
  level: "info"
  file_path: "logs"
  max_size: 20  # MB

environment: "development"
```

## 🧪 测试验证

### 快速测试

```bash
# 运行快速测试套件
./scripts/quick_test.sh
```

### 手动测试示例

#### 1. 生成用户令牌
```bash
curl "http://localhost:8000/api/auth/create_user_token?user_id=1001"
```

#### 2. 获取秒杀令牌
```bash
curl -X POST "http://localhost:8000/api/seckill/token?gid=1001" \
  -H "Authorization: <user_token>"
```

#### 3. 执行秒杀
```bash
curl -X POST "http://localhost:8000/api/seckill?gid=1001&token=<seckill_token>" \
  -H "Authorization: <user_token>"
```

#### 4. 管理功能（需要admin权限）
```bash
# 预加载库存
curl -X POST "http://localhost:8000/api/admin/preload/1001?admin=1"

# 设置秒杀开关
curl -X POST "http://localhost:8000/api/admin/config/seckill/enable?admin=1&enabled=true"

# 查看黑名单
curl "http://localhost:8000/api/admin/blacklist?admin=1"
```

## 📊 API接口文档

### 用户接口

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| `GET` | `/api/goods/:id` | 获取商品信息 | 否 |
| `POST` | `/api/seckill/token` | 获取秒杀令牌 | 是 |
| `POST` | `/api/seckill` | 执行秒杀 | 是 |
| `POST` | `/api/payment/simulate` | 模拟支付 | 是 |
| `GET` | `/api/auth/create_user_token` | 生成用户令牌 | 否 |
| `GET` | `/api/auth/verify_user_token` | 验证用户令牌 | 否 |

### 管理接口

| 方法 | 端点 | 描述 | 权限 |
|------|------|------|------|
| `POST` | `/api/admin/preload/:id` | 预加载库存 | admin |
| `POST` | `/api/admin/reset_db` | 重置数据库 | admin |
| `POST` | `/api/admin/config/seckill/enable` | 设置秒杀开关 | admin |
| `POST` | `/api/admin/config/rate_limit` | 设置限流配置 | admin |
| `POST` | `/api/admin/blacklist/add` | 添加黑名单 | admin |
| `GET` | `/api/admin/blacklist` | 获取黑名单 | admin |

## 🛡️ 核心防护机制

### 1. 分布式锁机制
- **基于Etcd**：强一致性分布式锁，防止集群脑裂
- **防死锁**：自动TTL过期机制
- **锁粒度控制**：用户级和商品级锁，减少竞争
- **快速失败**：锁获取超时立即返回，避免阻塞

### 2. 库存安全
- **Redis预减库存**：内存操作，高性能
- **数据库乐观锁**：版本号控制，数据一致性
- **失败恢复**：异常时自动恢复Redis库存
- **双重校验**：Redis + MySQL双重库存检查

### 3. 限流防护
- **用户级限流**：基于Redis+Lua脚本的原子操作
- **动态配置**：通过Etcd实时调整限流阈值
- **多维度限流**：IP、用户ID、商品ID等多个维度

### 4. 安全验证
- **令牌机制**：JWT-like用户令牌和秒杀令牌
- **黑名单**：恶意用户隔离
- **活动时间校验**：严格的秒杀时间控制
- **参数验证**：全面的输入参数校验

## ⚡ 性能指标

| 指标 | 预期值 | 说明 |
|------|--------|------|
| **单机QPS** | 10,000+ | 网关层处理能力 |
| **并发用户** | 5,000+ | 同时在线用户数 |
| **订单处理** | 1,000+ TPS | 下单事务处理能力 |
| **分布式锁性能** | < 5ms | 锁获取平均时间 |
| **数据一致性** | 99.99% | 零超卖保证 |
| **响应时间** | < 100ms | API平均响应时间 |

## 🔧 配置说明

### 动态配置（Etcd）

系统支持通过Etcd进行实时配置更新：

```bash
# 开启/关闭秒杀系统
curl -X POST "http://localhost:8000/api/admin/config/seckill/enable?admin=1&enabled=true"

# 设置用户限流（次/分钟）
curl -X POST "http://localhost:8000/api/admin/config/rate_limit?admin=1&limit=50"

# 添加用户到黑名单
curl -X POST "http://localhost:8000/api/admin/blacklist/add?admin=1&user_id=9999&reason=test"
```

## 🐛 故障排除

### 常见问题

1. **服务启动失败**
   - 检查端口占用：`netstat -tulpn | grep 8000`
   - 验证依赖服务状态：MySQL、Redis、Kafka、Etcd

2. **Redis连接失败**
   - 确认Redis集群所有节点正常运行
   - 检查防火墙设置

3. **分布式锁获取失败**
   - 检查Etcd服务状态
   - 查看锁竞争情况，调整锁超时时间

4. **性能问题**
   - 监控系统资源使用情况
   - 调整连接池配置
   - 检查慢查询日志

### 日志查看

```bash
# 查看应用日志
tail -f /var/log/seckill_system.log

# 查看测试日志
cat seckill_test.log

# 查看服务状态
systemctl status mysql
systemctl status redis
systemctl status kafka
systemctl status etcd
```
