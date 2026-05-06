<template>
  <div class="dashboard">
    <el-row :gutter="16">
      <el-col :span="24">
        <el-card class="box-card">
          <template #header>
            <div class="card-header">
              <span>实时行情</span>
              <el-tag :type="wsConnected ? 'success' : 'danger'">
                {{ wsConnected ? 'WebSocket 已连接' : '未连接' }}
              </el-tag>
            </div>
          </template>
          <el-table :data="tickers" stripe v-loading="loading">
            <el-table-column prop="exchange" label="交易所" width="100">
              <template #default="{ row }">
                <el-tag :type="getExchangeType(row.exchange)">{{ row.exchange }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="symbol" label="交易对" width="120" />
            <el-table-column prop="price" label="价格" width="150">
              <template #default="{ row }">
                ${{ row.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }}
              </template>
            </el-table-column>
            <el-table-column prop="bid" label="买一" width="120">
              <template #default="{ row }">
                ${{ row.bid.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
              </template>
            </el-table-column>
            <el-table-column prop="ask" label="卖一" width="120">
              <template #default="{ row }">
                ${{ row.ask.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
              </template>
            </el-table-column>
            <el-table-column prop="change24h" label="24h 涨跌%" width="100">
              <template #default="{ row }">
                <span :class="row.change24h >= 0 ? 'text-up' : 'text-down'">
                  {{ row.change24h >= 0 ? '+' : '' }}{{ row.change24h.toFixed(2) }}%
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="volume24h" label="24h 成交量" width="150">
              <template #default="{ row }">
                ${{ (row.volume24h / 1000000).toFixed(2) }}M
              </template>
            </el-table-column>
            <el-table-column prop="timestamp" label="更新时间" width="180">
              <template #default="{ row }">
                {{ formatTime(row.timestamp) }}
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top: 16px;">
      <el-col :span="12">
        <el-card class="box-card">
          <template #header>
            <span>套利机会</span>
          </template>
          <div class="opportunities">
            <div class="opportunity-item">
              <div class="opp-header">
                <el-tag type="warning">跨交易所套利</el-tag>
                <span class="opp-profit">+0.60%</span>
              </div>
              <div class="opp-detail">
                BTCUSDT: 币安 $67,500 → OKX $67,520
              </div>
            </div>
            <div class="opportunity-item">
              <div class="opp-header">
                <el-tag type="success">资金费率套利</el-tag>
                <span class="opp-profit">+8.5%/年</span>
              </div>
              <div class="opp-detail">
                ETHUSDT: 币安 0.01% → OKX 0.03%
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="box-card">
          <template #header>
            <span>快速操作</span>
          </template>
          <div class="quick-actions">
            <el-button type="primary" @click="$router.push('/strategies')">策略配置</el-button>
            <el-button type="success" @click="$router.push('/positions')">仓位管理</el-button>
            <el-button type="warning" @click="$router.push('/alerts')">告警中心</el-button>
            <el-button type="info" @click="$router.push('/settings')">系统设置</el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useTickerStore } from '@/stores/ticker'

const tickerStore = useTickerStore()
const { tickers, loading, wsConnected } = tickerStore

const getExchangeType = (exchange: string) => {
  const types: Record<string, any> = {
    binance: 'warning',
    okx: 'success',
    bitget: 'info',
  }
  return types[exchange] || ''
}

const formatTime = (timestamp: number) => {
  return new Date(timestamp).toLocaleTimeString()
}

onMounted(() => {
  tickerStore.fetchTickers()
  // tickerStore.connectWebSocket()
})
</script>

<style scoped>
.dashboard {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.text-up {
  color: #67c23a;
}

.text-down {
  color: #f56c6c;
}

.opportunities {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.opportunity-item {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
}

.opp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.opp-profit {
  font-size: 18px;
  font-weight: bold;
  color: #67c23a;
}

.opp-detail {
  font-size: 14px;
  color: #606266;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
</style>
