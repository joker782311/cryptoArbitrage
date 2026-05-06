# 部署指南

## 环境要求

- Docker 20+
- Docker Compose 2+
- 可用内存：至少 2GB

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/joker782311/cryptoArbitrage.git
cd cryptoArbitrage
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env` 文件，配置以下内容：

```bash
# 服务配置
SERVER_PORT=8080
SERVER_MODE=release  # 生产环境使用 release

# 数据库配置
DB_HOST=mysql
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_secure_password  # 修改为强密码
DB_NAME=crypto_arbitrage

# Redis 配置
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=  # 生产环境建议设置密码
```

### 3. 启动服务

```bash
docker-compose up -d
```

### 4. 查看运行状态

```bash
# 查看容器状态
docker-compose ps

# 查看应用日志
docker-compose logs -f app

# 查看数据库日志
docker-compose logs -f mysql
```

### 5. 停止服务

```bash
docker-compose down
```

## 生产环境部署

### 前置准备

1. **域名和 SSL 证书**

   如果使用 HTTPS，需要配置反向代理（如 Nginx）

2. **数据库备份**

   配置 MySQL 定时备份：

   ```bash
   # 备份脚本
   mysqldump -h mysql -u root -p crypto_arbitrage > backup_$(date +%Y%m%d).sql
   ```

3. **密钥管理**

   交易所 API Key 使用加密存储，不要明文保存在环境变量中

### Docker Compose 生产配置

创建 `docker-compose.prod.yml`：

```yaml
version: '3.8'

services:
  app:
    image: your-registry/crypto-arbitrage:latest
    ports:
      - "8080:8080"
    environment:
      - SERVER_MODE=release
      - DB_HOST=mysql
      - DB_PASSWORD=${DB_PASSWORD}
    restart: unless-stopped
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: crypto_arbitrage
    volumes:
      - mysql_data:/var/lib/mysql
      - ./mysql/conf.d:/etc/mysql/conf.d
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  mysql_data:
  redis_data:
```

### 监控和告警

1. **应用监控**
   - 集成 Prometheus + Grafana
   - 监控指标：请求延迟、错误率、CPU/内存使用

2. **日志收集**
   - 使用 ELK Stack 或 Loki
   - 集中管理应用日志

3. **告警配置**
   - 在系统中配置 Telegram/Slack 告警
   - 设置关键指标阈值

## 故障排查

### 容器无法启动

```bash
# 查看详细日志
docker-compose logs app

# 检查配置
docker-compose config
```

### 数据库连接失败

```bash
# 测试数据库连接
docker-compose exec app ping mysql

# 检查 MySQL 状态
docker-compose exec mysql mysqladmin status -u root -p
```

### 数据持久化

确保 volume 正确配置：

```bash
# 查看 volume
docker volume ls

# 检查数据目录
docker-compose exec mysql ls /var/lib/mysql
```

## 更新部署

```bash
# 拉取最新代码
git pull

# 重新构建并重启
docker-compose up -d --build
```
