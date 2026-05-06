# Crypto Arbitrage Platform 部署指南

## 目录

1. [环境要求](#环境要求)
2. [快速开始](#快速开始)
3. [生产环境部署](#生产环境部署)
4. [监控和告警](#监控和告警)
5. [备份和恢复](#备份和恢复)
6. [故障排查](#故障排查)

## 环境要求

| 组件 | 最低要求 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核心 | 4 核心 |
| 内存 | 4GB | 8GB |
| 磁盘 | 20GB SSD | 50GB+ SSD |
| Docker | 20.10+ | 24.0+ |
| Docker Compose | 2.0+ | 2.20+ |

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/joker782311/cryptoArbitrage.git
cd cryptoArbitrage
```

### 2. 配置环境变量

```bash
cat > .env << EOF
DB_PASSWORD=YourSecurePassword123!
REDIS_PASSWORD=YourRedisPassword456!
SERVER_PORT=8080
SERVER_MODE=release
EOF
```

### 3. 启动服务

```bash
docker-compose up -d
docker-compose ps
docker-compose logs -f app
```

### 4. 验证部署

```bash
curl http://localhost:8080/health
```

### 5. 访问应用

- 前端：http://localhost
- API: http://localhost:8080/api/v1

## 生产环境部署

### 服务器初始化

```bash
# 安装 Docker
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER

# 安装 Docker Compose
sudo apt install docker-compose-plugin -y

# 配置防火墙
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### SSL/HTTPS 配置

```bash
sudo apt install certbot python3-certbot-nginx -y
sudo certbot --nginx -d your-domain.com
```

## 监控和告警

### Prometheus 配置

```yaml
# configs/prometheus.yml
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'arbitrage'
    static_configs:
      - targets: ['app:8080']
```

### Grafana Dashboard

导入 Dashboard ID: `10826` (Go 应用监控)

## 备份和恢复

### 数据库备份

```bash
docker-compose exec -T mysql mysqldump -u root -p${DB_PASSWORD} crypto_arbitrage > backup_$(date +%Y%m%d).sql
```

### 数据恢复

```bash
cat backup_20240101.sql | docker-compose exec -T mysql mysql -u root -p${DB_PASSWORD} crypto_arbitrage
```

## 故障排查

### 容器无法启动

```bash
docker-compose logs app
docker stats
docker-compose restart app
```

### 数据库连接超时

```bash
docker-compose exec app ping mysql
docker-compose exec mysql mysqladmin status -u root -p
```

### 内存不足

```yaml
# docker-compose.yml 中添加:
services:
  app:
    deploy:
      resources:
        limits:
          memory: 2G
```

### 端口冲突

```bash
sudo lsof -i :8080
# 修改 .env: SERVER_PORT=8081
```

## 安全加固

1. 修改默认密码
2. 配置 API Key IP 白名单
3. 定期轮换密钥
4. 限制数据库访问

## 性能调优

```bash
# MySQL 优化 configs/mysql.cnf
[mysqld]
max_connections = 500
innodb_buffer_pool_size = 1G

# Redis 优化 configs/redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
```
