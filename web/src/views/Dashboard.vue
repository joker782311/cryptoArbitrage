<template>
  <div class="dashboard-container">
    <!-- 实时行情表格 -->
    <el-card class="main-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <span class="card-title">实时行情</span>
            <el-tag :type="wsConnected ? 'success' : 'danger'">
              {{ wsConnected ? 'WebSocket 已连接' : '未连接' }}
            </el-tag>
          </div>
          <div class="header-right">
            <el-select v-model="selectedExchanges" multiple placeholder="筛选交易所" size="default" clearable>
              <el-option label="币安 (Binance)" value="binance" />
              <el-option label="OKX" value="okx" />
              <el-option label="Bitget" value="bitget" />
            </el-select>
          </div>
        </div>
      </template>
      <el-table :data="paginatedTickers" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="exchange" label="交易所" min-width="100">
          <template #default="{ row }">
            <el-tag :type="getExchangeType(row.exchange)">{{ row.exchange }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="symbol" label="交易对" min-width="120" />
        <el-table-column prop="price" label="价格" min-width="150">
          <template #default="{ row }">
            <span class="price-text">${{ row.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="bid" label="买一" min-width="120">
          <template #default="{ row }">
            ${{ row.bid.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
          </template>
        </el-table-column>
        <el-table-column prop="ask" label="卖一" min-width="120">
          <template #default="{ row }">
            ${{ row.ask.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
          </template>
        </el-table-column>
        <el-table-column prop="change24h" label="24h 涨跌%" min-width="120">
          <template #default="{ row }">
            <span :class="row.change24h >= 0 ? 'text-up' : 'text-down'">
              {{ row.change24h >= 0 ? '+' : '' }}{{ row.change24h.toFixed(2) }}%
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="volume24h" label="24h 成交量" min-width="150">
          <template #default="{ row }">
            ${{ (row.volume24h / 1000000).toFixed(2) }}M
          </template>
        </el-table-column>
        <el-table-column prop="high24h" label="24h 最高" min-width="120">
          <template #default="{ row }">
            ${{ row.high24h.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
          </template>
        </el-table-column>
        <el-table-column prop="low24h" label="24h 最低" min-width="120">
          <template #default="{ row }">
            ${{ row.low24h.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
          </template>
        </el-table-column>
        <el-table-column prop="timestamp" label="更新时间" min-width="160">
          <template #default="{ row }">
            {{ formatTime(row.timestamp) }}
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-container">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="pageSizes"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <!-- 套利机会和快速操作 -->
    <el-row :gutter="16" style="margin-top: 0;">
      <el-col :span="24">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-card class="side-card" shadow="never">
              <template #header>
                <div class="card-header">
                  <span class="card-title">套利机会</span>
                  <el-badge :value="2" type="primary" />
                </div>
              </template>
              <div class="opportunities">
                <div class="opportunity-item">
                  <div class="opp-header">
                    <el-tag type="warning">跨交易所套利</el-tag>
                    <span class="opp-profit">+0.60%</span>
                  </div>
                  <div class="opp-detail">
                    <span class="symbol">BTCUSDT</span>: 币安 <span class="price">$67,500</span> → OKX <span class="price">$67,520</span>
                  </div>
                </div>
                <div class="opportunity-item">
                  <div class="opp-header">
                    <el-tag type="success">资金费率套利</el-tag>
                    <span class="opp-profit">+8.5%/年</span>
                  </div>
                  <div class="opp-detail">
                    <span class="symbol">ETHUSDT</span>: 币安 <span class="rate">0.01%</span> → OKX <span class="rate">0.03%</span>
                  </div>
                </div>
                <div class="opportunity-item">
                  <div class="opp-header">
                    <el-tag type="danger">期现套利</el-tag>
                    <span class="opp-profit">+12.3%/年</span>
                  </div>
                  <div class="opp-detail">
                    <span class="symbol">BTCUSDT</span>: 现货 <span class="price">$67,500</span> → 合约 <span class="price">$68,200</span>
                  </div>
                </div>
              </div>
            </el-card>
          </el-col>

          <el-col :span="16">
            <el-card class="side-card" shadow="never">
              <template #header>
                <span class="card-title">快速操作</span>
              </template>
              <div class="quick-actions">
                <el-button type="primary" @click="$router.push('/strategies')">策略配置</el-button>
                <el-button type="success" @click="$router.push('/positions')">仓位管理</el-button>
                <el-button type="warning" @click="$router.push('/alerts')">告警中心</el-button>
                <el-button type="info" @click="$router.push('/settings')">系统设置</el-button>
              </div>
              <el-divider />
              <div class="account-overview">
                <el-row :gutter="16">
                  <el-col :span="8">
                    <div class="stat-item">
                      <div class="stat-label">总仓位</div>
                      <div class="stat-value">$0</div>
                    </div>
                  </el-col>
                  <el-col :span="8">
                    <div class="stat-item">
                      <div class="stat-label">24h 盈亏</div>
                      <div class="stat-value profit">$0</div>
                    </div>
                  </el-col>
                  <el-col :span="8">
                    <div class="stat-item">
                      <div class="stat-label">可用余额</div>
                      <div class="stat-value">$0</div>
                    </div>
                  </el-col>
                </el-row>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useTickerStore } from '@/stores/ticker'
import { storeToRefs } from 'pinia'

const tickerStore = useTickerStore()
const { loading, wsConnected } = tickerStore
const { tickers } = storeToRefs(tickerStore)

// 分页配置
const currentPage = ref(1)
const pageSize = ref(10)
const pageSizes = [10, 20, 50, 100]

// 交易所筛选
const selectedExchanges = ref<string[]>([])

// 筛选后的数据
const filteredTickers = computed(() => {
  if (selectedExchanges.value.length === 0) return tickers.value
  return tickers.value.filter(t => selectedExchanges.value.includes(t.exchange))
})

// 分页后的数据
const paginatedTickers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredTickers.value.slice(start, end)
})

// 总数
const total = computed(() => filteredTickers.value.length)

// 分页事件处理
const handleSizeChange = (val: number) => {
  pageSize.value = val
  currentPage.value = 1
}

const handleCurrentChange = (val: number) => {
  currentPage.value = val
}

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
})
</script>

<style scoped>
.dashboard-container {
  padding: 16px;
  width: 100%;
}

.main-card,
.side-card {
  width: 100%;
}

.main-card :deep(.el-card__header),
.side-card :deep(.el-card__header) {
  padding: 16px;
  background: #fff;
  border-bottom: 1px solid #ebeef5;
}

.main-card :deep(.el-card__body) {
  padding: 0;
  background: #fff;
}

.side-card :deep(.el-card__body) {
  padding: 16px;
  background: #fff;
}

/* 强制表格撑满容器 */
.main-card :deep(.el-table) {
  width: 100%;
  table-layout: fixed !important;
}

.main-card :deep(.el-table__body-wrapper) {
  overflow-x: auto;
}

.pagination-container {
  padding: 16px;
  display: flex;
  justify-content: flex-end;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-right {
  display: flex;
  align-items: center;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.price-text {
  font-weight: 600;
}

.text-up {
  color: #67c23a;
}

.text-down {
  color: #f56c6c;
}

.opportunities {
  padding: 4px 0;
}

.opportunity-item {
  padding: 14px 16px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 12px;
}

.opportunity-item:last-child {
  margin-bottom: 0;
}

.opp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
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

.opp-detail .symbol {
  font-weight: 600;
  color: #303133;
}

.opp-detail .price,
.opp-detail .rate {
  color: #409eff;
  font-family: 'Courier New', monospace;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.quick-actions .el-button {
  flex: 1;
  min-width: 100px;
}

.account-overview {
  padding-top: 16px;
}

.stat-item {
  text-align: center;
  padding: 12px 8px;
  background: #f0f9ff;
  border-radius: 6px;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-bottom: 6px;
}

.stat-value {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.stat-value.profit {
  color: #67c23a;
}
</style>
