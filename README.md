# Crypto Arbitrage Platform

币圈套利交易平台 - 支持 CEX 和 DEX 多种套利策略

## 功能特性

- **CEX 套利**: 跨交易所套利、资金费率套利、期现套利、三角套利
- **DEX 套利**: 三角套利、跨 DEX 套利
- **支持的交易所**: 币安、OKX、Bitget
- **支持的链**: Ethereum、BSC、Polygon、Arbitrum、Optimism、Base、Solana

## 技术栈

- 后端：Go 1.21+
- 前端：Vue 3 + TypeScript
- 数据库：MySQL 8
- 缓存：Redis 7
- 部署：Docker

## 快速开始

### 环境要求

- Go 1.21+
- Docker & Docker Compose
- Node.js 18+ (前端开发)

### 本地开发

1. 启动数据库和 Redis

```bash
docker-compose up -d mysql redis
```

2. 运行后端

```bash
cp .env.example .env
# 编辑 .env 配置
go run ./cmd/main.go
```

3. 前端开发

```bash
cd web
npm install
npm run dev
```

### Docker 部署

```bash
docker-compose up -d
```

## 项目结构

```
├── cmd/                 # 应用入口
├── internal/            # 内部包
│   ├── config/         # 配置
│   ├── database/       # 数据库连接
│   ├── model/          # 数据模型
│   ├── exchange/       # 交易所接口
│   ├── strategy/       # 策略引擎
│   └── service/        # 业务服务
├── pkg/                # 公共包
│   └── logger/         # 日志
├── web/                # 前端
└── docs/               # 文档
```

## License

MIT
