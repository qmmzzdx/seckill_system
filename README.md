# 秒杀系统项目

## 🎯 项目概述
一个基于Go语言的高并发秒杀系统，采用微服务架构思想，集成了多种中间件来保证系统的高性能、高可用和安全性。系统通过分布式锁、缓存预减、异步处理等机制，有效解决了高并发场景下的超卖、性能瓶颈和数据一致性问题。

## 🏗️ 系统架构

### 核心架构图
```
客户端 → Gateway网关 → 业务处理层 → 数据访问层 → 存储层
                    ↓
              消息队列(Kafka) → 异步处理
                    ↓
              配置中心(Etcd) → 动态配置 + 分布式锁
```

### 技术栈
- **语言**: Go 1.24+
- **Web框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL
- **缓存**: Redis Cluster (6节点集群)
- **消息队列**: Kafka (3节点集群)
- **配置中心 & 分布式锁**: Etcd
- **测试框架**: Go Testing + Mock
- **部署**: 单机部署（支持水平扩展）

## 📁 项目结构

```
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

## 🔄 核心业务流程

### 1. 秒杀令牌获取流程
```
用户认证 → 检查秒杀开关 → 黑名单检查 → 商品验证 → 
活动时间检查 → 库存检查 → 限流检查 → 生成令牌
```

### 2. 秒杀下单流程（分布式锁保护）
```
获取分布式锁 → 令牌验证 → Redis预减库存 → 数据库事务 → 
乐观锁扣库存 → 创建订单 → 发送Kafka消息 → 释放分布式锁
```

### 3. 支付流程
```
支付请求 → 支付处理 → 发送支付消息 → 
异步更新订单状态 → 失败时恢复库存
```

## 🛡️ 安全与防护机制

### 1. 分布式锁机制
- **Etcd分布式锁**：基于租约和事务的强一致性锁
- **防死锁**：自动TTL过期机制
- **锁释放保证**：defer确保锁一定释放
- **锁粒度控制**：商品级锁防止超卖

### 2. 用户认证
- JWT-like令牌机制
- Redis存储用户会话
- 令牌自动过期

### 3. 限流防护
- 基于Redis+Lua的用户级限流
- 可动态配置的限流阈值
- 防止恶意请求

### 4. 库存安全
- Redis预减库存防止超卖
- 数据库乐观锁保证数据一致性
- 失败时库存自动恢复
- **分布式锁双重保护**：确保库存扣减的原子性

### 5. 黑名单机制
- Etcd存储黑名单信息
- 支持自动过期
- 动态管理恶意用户

## ⚡ 性能优化特性

### 1. 分布式并发控制
- **Etcd分布式锁**：强一致性，防止集群脑裂
- **锁竞争优化**：快速失败机制，避免长时间等待
- **锁超时控制**：防止死锁，自动释放

### 2. 缓存策略
- Redis集群(6节点)缓存商品库存
- 热点数据预加载
- 多级缓存架构

### 3. 异步处理
- Kafka集群(3节点)消息队列解耦
- 订单创建与后续处理异步化
- 支付结果异步通知

### 4. 数据库优化
- 连接池配置
- 事务管理
- 乐观锁并发控制

## 🚀 快速部署

### 一键部署所有依赖服务
```bash
# 授予执行权限
chmod +x run_services.sh
chmod +x scripts/*/*.sh

# 一键启动所有服务
bash ./run_services.sh
```

### 分步部署

#### 1. 安装MySQL
```bash
./scripts/mysql/install_mysql.sh
```

#### 2. 安装Redis集群(6节点)
```bash
./scripts/redis/install_redis.sh
```

#### 3. 安装Kafka集群(3节点)
```bash
./scripts/kafka/install_kafka.sh
```

#### 4. 安装Etcd
```bash
./scripts/etcd/install_etcd.sh
```

#### 5. 启动应用
```bash
go run cmd/gateway/main.go
```

### 一键执行测试
```bash
# 授予执行权限
chmod +x scripts/quick_test.sh
chmod +x scripts/test_seckill.sh

# 一键测试秒杀系统
bash ./scripts/quick_test.sh
```

### 集群配置详情

#### Redis集群配置
- **节点数量**: 6节点 (3主3从)
- **端口范围**: 7000-7005
- **集群模式**: 自动分片和数据复制
- **持久化**: RDB + AOF

#### Kafka集群配置
- **节点数量**: 3节点
- **端口**: 9092, 9094, 9096
- **副本因子**: 3
- **分区数**: 可配置

#### Etcd配置
- **单节点模式**: 生产环境建议3节点集群
- **数据持久化**: 启用
- **客户端端口**: 2379
- **对等端口**: 2380

## 🧪 测试与验证

### 运行测试套件
```bash
# 运行所有测试
go test ./test/... -v

# 运行分布式锁专项测试
go test ./test/... -run TestDistributedLock -v

# 测试覆盖率报告
go test ./test/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 测试场景覆盖
- ✅ 正常秒杀流程
- ✅ 分布式锁并发安全(200+并发)
- ✅ 库存超卖防护
- ✅ 系统限流验证
- ✅ 黑名单机制
- ✅ 异常情况处理
- ✅ 性能基准测试

## 🔧 配置管理

### 动态配置（通过Etcd）
- 秒杀系统开关
- 限流阈值配置
- 黑名单管理
- 实时配置更新监听
- **分布式锁配置**：TTL时间、重试策略

### 静态配置（conf/conf.yaml）
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
```

## 📊 API接口

### 用户接口
- `GET /api/goods/:id` - 获取商品信息
- `POST /api/seckill/token` - 获取秒杀令牌
- `POST /api/seckill` - 执行秒杀（受分布式锁保护）
- `POST /api/payment/simulate` - 模拟支付

### 管理接口
- `POST /api/admin/preload/:id` - 预加载库存
- `POST /api/admin/config/seckill/enable` - 秒杀开关
- `POST /api/admin/config/rate_limit` - 限流配置
- `POST /api/admin/blacklist/add` - 添加黑名单
- `GET /api/admin/blacklist` - 查看黑名单

## 🎯 核心价值

本项目通过**Etcd分布式锁**的深度集成，解决了秒杀系统最核心的**超卖问题**，同时保持了系统的高性能和可扩展性。完整的测试套件和自动化部署脚本确保了系统的稳定性和易用性，为高并发场景下的电商系统提供了生产就绪的解决方案。

## 📈 性能预期

在当前架构下，系统预计可以支持：
- **单机QPS**: 10,000+
- **并发用户**: 5,000+
- **订单处理**: 1,000+ TPS
- **数据一致性**: 99.99%
- **分布式锁性能**: < 5ms获取时间
- **并发安全**: 200+并发下零超卖

## 🔮 扩展方向

1. **监控告警**: 集成Prometheus + Grafana监控锁竞争情况
2. **分布式追踪**: 集成Jaeger追踪锁获取和业务处理链路
3. **容器化**: Docker + Kubernetes部署
4. **数据库分库分表**: 应对更大数据量
5. **多级缓存**: 引入本地缓存减少Redis压力
6. **限流升级**: 集成Sentinel等专业限流组件
7. **锁优化**: 实现可重入锁、读写锁等高级锁特性

## 📝 使用说明

1. **环境要求**: Ubuntu Linux，Go 1.24+
2. **依赖安装**: 使用提供的脚本一键安装所有依赖
3. **配置调整**: 根据实际环境修改conf/目录下的配置文件
4. **服务启动**: 使用run_services.sh启动所有服务
5. **测试验证**: 运行测试套件验证系统功能
6. **压力测试**: 可使用JMeter等工具进行性能测试

项目提供了完整的部署脚本和配置，方便从零开始部署整个秒杀系统环境
