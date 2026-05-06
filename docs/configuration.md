# 配置说明

## 环境变量

### 服务配置

| 变量名 | 说明 | 默认值 | 选项 |
|--------|------|--------|------|
| `SERVER_PORT` | 服务监听端口 | `8080` | - |
| `SERVER_MODE` | 运行模式 | `debug` | `debug`, `release` |

### 数据库配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `DB_HOST` | MySQL 主机地址 | `localhost` |
| `DB_PORT` | MySQL 端口 | `3306` |
| `DB_USER` | 数据库用户名 | `root` |
| `DB_PASSWORD` | 数据库密码 | - |
| `DB_NAME` | 数据库名称 | `crypto_arbitrage` |

### Redis 配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `REDIS_HOST` | Redis 主机地址 | `localhost` |
| `REDIS_PORT` | Redis 端口 | `6379` |
| `REDIS_PASSWORD` | Redis 密码 | - |

## 策略配置

策略通过数据库 `strategies` 表配置，支持以下字段：

### 通用配置

| 字段 | 说明 | 类型 | 示例 |
|------|------|------|------|
| `name` | 策略名称 | string | `cross_exchange` |
| `is_enabled` | 是否启用 | boolean | `true` |
| `auto_execute` | 是否自动执行 | boolean | `false` |
| `min_profit_rate` | 最小利润率 | decimal | `0.005` (0.5%) |
| `max_position` | 最大仓位 | decimal | `10000` (USDT) |
| `stop_loss_rate` | 止损率 | decimal | `0.02` (2%) |
| `config` | JSON 扩展配置 | text | `{"exchanges":["binance","okx"]}` |

### 策略名称列表

| 策略名称 | 说明 |
|----------|------|
| `cross_exchange` | 跨交易所套利 |
| `funding_rate` | 资金费率套利 |
| `spot_future` | 期现套利 |
| `triangular` | 三角套利 (CEX) |
| `dex_triangular` | DEX 三角套利 |
| `dex_cross_dex` | 跨 DEX 套利 |

## 告警配置

告警通过 `alert_configs` 表配置：

| 字段 | 说明 | 选项 |
|------|------|------|
| `channel` | 告警渠道 | `telegram`, `slack`, `email`, `webhook` |
| `webhook_url` | Webhook URL | - |
| `email` | 邮箱地址 | - |
| `chat_id` | Telegram Chat ID | - |
| `is_enabled` | 是否启用 | `true/false` |

### Telegram 配置示例

1. 创建 Telegram Bot：通过 @BotFather 获取 `bot_token`
2. 获取 Chat ID：发送消息给 bot，访问 `https://api.telegram.org/bot<token>/getUpdates`
3. 配置到数据库：

```sql
INSERT INTO alert_configs (channel, chat_id, is_enabled)
VALUES ('telegram', 'your_chat_id', true);
```

### Webhook 配置示例

```sql
INSERT INTO alert_configs (channel, webhook_url, is_enabled)
VALUES ('webhook', 'https://your-server.com/webhook', true);
```

## 交易所 API Key 配置

API Key 加密存储在 `api_keys` 表：

```sql
INSERT INTO api_keys (exchange, name, api_key, api_secret, passphrase, is_enabled)
VALUES (
  'binance',
  'main_account',
  'encrypted_api_key',
  'encrypted_secret',
  NULL,
  true
);
```

### 各交易所要求

| 交易所 | API Key | Secret | Passphrase |
|--------|---------|--------|------------|
| Binance | ✓ | ✓ | - |
| OKX | ✓ | ✓ | ✓ |
| Bitget | ✓ | ✓ | ✓ |

### API Key 权限要求

- **行情读取**：读取行情数据（必需）
- **账户读取**：查询余额、仓位（必需）
- **交易权限**：下单、撤单（自动交易需要）

**安全建议：**
- 不要开启提现权限
- 设置 IP 白名单
- 定期轮换 API Key

## 配置示例

### 最小可行配置

```bash
# .env
SERVER_PORT=8080
DB_HOST=localhost
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=crypto_arbitrage
REDIS_HOST=localhost
REDIS_PORT=6379
```

### 生产环境配置

```bash
# .env.prod
SERVER_PORT=8080
SERVER_MODE=release

DB_HOST=10.0.0.10
DB_PORT=3306
DB_USER=arbitrage_user
DB_PASSWORD=strong_password_here
DB_NAME=crypto_arbitrage

REDIS_HOST=10.0.0.11
REDIS_PORT=6379
REDIS_PASSWORD=redis_password_here
```

### Docker Compose 配置

```yaml
# docker-compose.yml
services:
  app:
    environment:
      - SERVER_MODE=release
      - DB_HOST=mysql
      - DB_PASSWORD=${DB_PASSWORD}
```
