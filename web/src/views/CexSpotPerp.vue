<template>
  <div class="spot-perp-page">
    <div class="page-header">
      <div>
        <h2>跨所期现模拟盘</h2>
        <div class="subtitle">
          Binance / OKX / Bitget · 现货与 U 本位永续组合扫描 · 行情更新时间 {{ formatTs(lastQuoteAt) }}
        </div>
      </div>
      <div class="header-actions">
        <el-tag :type="status === 'running' ? 'success' : 'danger'" size="large">
          {{ status === 'running' ? '运行中' : '已熔断' }}
        </el-tag>
        <el-tag :type="wsConnected ? 'success' : 'warning'" size="large">
          {{ wsConnected ? 'WebSocket 实时' : 'WebSocket 重连中' }}
        </el-tag>
        <el-button :icon="Refresh" @click="store.fetchSimulation()">刷新</el-button>
        <el-button v-if="status === 'running'" type="danger" :icon="SwitchButton" @click="handleHalt">
          熔断清仓
        </el-button>
        <el-button v-else type="primary" :icon="VideoPlay" @click="store.resumeSimulation()">
          恢复模拟
        </el-button>
      </div>
    </div>

    <el-alert
      v-if="status === 'halted'"
      class="halt-alert"
      type="error"
      :closable="false"
      show-icon
      :title="`熔断原因：${haltReason}`"
    />
    <el-alert
      v-if="Object.keys(marketErrors).length > 0"
      class="halt-alert"
      type="warning"
      :closable="false"
      show-icon
      :title="`部分行情源更新失败：${Object.keys(marketErrors).length} 项`"
    />

    <el-row :gutter="16" class="summary-row">
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">累计净收益</div>
          <div :class="pnl.totalPnL >= 0 ? 'metric-value profit' : 'metric-value loss'">
            {{ signedMoney(pnl.totalPnL) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">已实现收益</div>
          <div :class="pnl.realizedPnL >= 0 ? 'metric-value profit' : 'metric-value loss'">
            {{ signedMoney(pnl.realizedPnL) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">未实现预估</div>
          <div :class="pnl.unrealizedPnL >= 0 ? 'metric-value profit' : 'metric-value loss'">
            {{ signedMoney(pnl.unrealizedPnL) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">持仓本金 / 冻结保证金</div>
          <div class="metric-value compact">{{ money(pnl.openNotional) }} / {{ money(totalFrozenUsdt) }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="summary-row secondary">
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">可用现货 USDT</div>
          <div class="metric-value small-value">{{ money(totalSpotUsdt) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">合约保证金账户</div>
          <div class="metric-value small-value">{{ money(totalPerpUsdt) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">可执行机会</div>
          <div class="metric-value small-value">{{ readyOpportunities.length }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-tabs model-value="opportunities" class="main-tabs">
      <el-tab-pane label="机会扫描" name="opportunities">
        <el-table :data="opportunities" stripe v-loading="loading" class="data-table">
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="方向" min-width="170">
            <template #default="{ row }">
              <el-tag :type="row.direction === 'spot_long_perp_short' ? 'success' : 'warning'">
                {{ directionText(row.direction) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="组合" min-width="180">
            <template #default="{ row }">
              <span class="pair-text">{{ row.spotExchange }} 现货 / {{ row.perpExchange }} 永续</span>
            </template>
          </el-table-column>
          <el-table-column label="现货价" min-width="120">
            <template #default="{ row }">{{ money(row.spotPrice) }}</template>
          </el-table-column>
          <el-table-column label="永续价" min-width="120">
            <template #default="{ row }">{{ money(row.perpPrice) }}</template>
          </el-table-column>
          <el-table-column label="基差收益" min-width="110">
            <template #default="{ row }">
              <span :class="row.basisAmount >= 0 ? 'text-up' : 'text-down'">{{ signedMoney(row.basisAmount) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="资金费" min-width="100">
            <template #default="{ row }">
              <span :class="row.fundingAmount >= 0 ? 'text-up' : 'text-down'">{{ signedMoney(row.fundingAmount) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="成本" min-width="130">
            <template #default="{ row }">{{ money(row.feeCost + row.slippage + row.safetyBuffer) }}</template>
          </el-table-column>
          <el-table-column label="净收益" min-width="120">
            <template #default="{ row }">
              <span :class="row.netProfit >= 0 ? 'text-up strong' : 'text-down strong'">{{ signedMoney(row.netProfit) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="收益率" min-width="90">
            <template #default="{ row }">{{ row.profitRate.toFixed(2) }}%</template>
          </el-table-column>
          <el-table-column label="状态" min-width="120">
            <template #default="{ row }">
              <el-tooltip v-if="row.status === 'blocked'" :content="row.blockReason" placement="top">
                <el-tag type="info">不可执行</el-tag>
              </el-tooltip>
              <el-tag v-else type="success">可执行</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button
                type="primary"
                size="small"
                :icon="CaretRight"
                :disabled="row.status !== 'ready' || status === 'halted'"
                @click="store.executeOpportunity(row)"
              >
                模拟开仓
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="虚拟资金" name="accounts">
        <el-table :data="accounts" stripe class="data-table">
          <el-table-column prop="exchange" label="交易所" min-width="110">
            <template #default="{ row }">
              <el-tag :type="exchangeType(row.exchange)">{{ row.exchange }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="现货 USDT" min-width="130">
            <template #default="{ row }">{{ money(row.usdt) }}</template>
          </el-table-column>
          <el-table-column label="合约 USDT" min-width="130">
            <template #default="{ row }">{{ money(row.perpUsdt) }}</template>
          </el-table-column>
          <el-table-column label="冻结保证金" min-width="130">
            <template #default="{ row }">{{ money(row.frozenUsdt) }}</template>
          </el-table-column>
          <el-table-column label="现货库存" min-width="260">
            <template #default="{ row }">
              <div class="asset-tags">
                <el-tag v-for="(amount, asset) in row.spotBalances" :key="asset" type="info">
                  {{ asset }} {{ amount.toFixed(4) }}
                </el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="永续持仓" min-width="260">
            <template #default="{ row }">
              <div class="asset-tags">
                <el-tag v-for="(amount, symbol) in row.perpPositions" :key="symbol" :type="amount >= 0 ? 'success' : 'danger'">
                  {{ symbol }} {{ amount.toFixed(4) }}
                </el-tag>
                <span v-if="Object.keys(row.perpPositions).length === 0" class="muted">无</span>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="模拟持仓" name="positions">
        <el-table :data="positions" stripe class="data-table">
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="方向" min-width="170">
            <template #default="{ row }">{{ directionText(row.direction) }}</template>
          </el-table-column>
          <el-table-column label="组合" min-width="180">
            <template #default="{ row }">{{ row.spotExchange }} / {{ row.perpExchange }}</template>
          </el-table-column>
          <el-table-column label="数量" min-width="120">
            <template #default="{ row }">{{ row.quantity.toFixed(6) }}</template>
          </el-table-column>
          <el-table-column label="本金" min-width="120">
            <template #default="{ row }">{{ money(row.notional) }}</template>
          </el-table-column>
          <el-table-column label="保证金" min-width="120">
            <template #default="{ row }">{{ money(row.margin) }}</template>
          </el-table-column>
          <el-table-column label="预计净收益" min-width="130">
            <template #default="{ row }">
              <span class="text-up strong">{{ signedMoney(row.netProfit) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="已实现收益" min-width="130">
            <template #default="{ row }">
              <span :class="row.realizedPnL >= 0 ? 'text-up strong' : 'text-down strong'">
                {{ row.status === 'closed' ? signedMoney(row.realizedPnL) : '-' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="开仓时间" min-width="180">
            <template #default="{ row }">{{ formatTs(row.openedAt) }}</template>
          </el-table-column>
          <el-table-column prop="closedAt" label="平仓时间" min-width="180">
            <template #default="{ row }">
              <span class="muted">{{ row.closedAt ? formatTs(row.closedAt) : '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" min-width="100">
            <template #default="{ row }">
              <el-tag :type="positionStatusType(row.status)">
                {{ positionStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button
                size="small"
                type="warning"
                :icon="CloseBold"
                :disabled="row.status !== 'open'"
                @click="handleClosePosition(row.id)"
              >
                模拟平仓
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="熔断动作" name="close-actions">
        <el-table :data="closeActions" stripe class="data-table">
          <el-table-column prop="positionId" label="持仓 ID" min-width="180" />
          <el-table-column prop="reason" label="原因" min-width="180" />
          <el-table-column label="现货腿" min-width="140">
            <template #default="{ row }">{{ sideText(row.spotAction) }}</template>
          </el-table-column>
          <el-table-column label="永续腿" min-width="160">
            <template #default="{ row }">{{ sideText(row.perpAction) }}</template>
          </el-table-column>
          <el-table-column label="生成时间" min-width="180">
            <template #default="{ row }">{{ formatTs(row.createdAt) }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessageBox } from 'element-plus'
import { CaretRight, CloseBold, Refresh, SwitchButton, VideoPlay } from '@element-plus/icons-vue'
import { useCexSpotPerpStore, type SpotPerpDirection } from '@/stores/cexSpotPerp'

const store = useCexSpotPerpStore()
const {
  loading,
  wsConnected,
  status,
  haltReason,
  accounts,
  opportunities,
  positions,
  closeActions,
  lastQuoteAt,
  marketErrors,
  pnl,
  totalSpotUsdt,
  totalPerpUsdt,
  totalFrozenUsdt,
  readyOpportunities,
} = storeToRefs(store)

onMounted(() => {
  store.fetchSimulation()
  store.connectWebSocket()
})

onUnmounted(() => {
  store.disconnectWebSocket()
})

const directionText = (direction: SpotPerpDirection) => {
  return direction === 'spot_long_perp_short' ? '买现货 + 空永续' : '卖库存 + 多永续'
}

const exchangeType = (exchange: string) => {
  const types: Record<string, string> = {
    binance: 'warning',
    okx: 'success',
    bitget: 'info',
  }
  return types[exchange] || ''
}

const money = (value: number) => {
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

const signedMoney = (value: number) => {
  return `${value >= 0 ? '+' : '-'}${money(Math.abs(value))}`
}

const handleHalt = async () => {
  await ElMessageBox.confirm('熔断会停止新开仓，并为所有模拟持仓生成紧急平仓动作。', '确认熔断', {
    type: 'warning',
    confirmButtonText: '熔断清仓',
    cancelButtonText: '取消',
  })
  await store.triggerCircuitBreaker('手动熔断')
}

const positionStatusText = (status: string) => {
  const text: Record<string, string> = {
    open: '持仓中',
    closing: '平仓中',
    closed: '已平仓',
  }
  return text[status] || status
}

const positionStatusType = (status: string) => {
  const type: Record<string, string> = {
    open: 'success',
    closing: 'warning',
    closed: 'info',
  }
  return type[status] || 'info'
}

const handleClosePosition = async (positionId: string) => {
  await ElMessageBox.confirm('模拟平仓会反向处理现货腿和永续腿，并释放冻结保证金。', '确认平仓', {
    type: 'warning',
    confirmButtonText: '模拟平仓',
    cancelButtonText: '取消',
  })
  await store.closePosition(positionId)
}

const sideText = (side: string) => {
  const text: Record<string, string> = {
    buy: '买入',
    sell: '卖出',
  }
  return text[side] || side
}

const formatTs = (value?: number) => {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}
</script>

<style scoped>
.spot-perp-page {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

h2 {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 650;
  color: #1f2937;
}

.subtitle,
.muted {
  color: #6b7280;
  font-size: 13px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.halt-alert,
.summary-row {
  margin-bottom: 16px;
}

.summary-row.secondary {
  margin-top: -8px;
}

.metric-card {
  min-height: 92px;
  margin-bottom: 12px;
}

.metric-card.small {
  min-height: 72px;
}

.metric-label {
  color: #6b7280;
  font-size: 13px;
  margin-bottom: 8px;
}

.metric-value {
  color: #111827;
  font-size: 24px;
  font-weight: 700;
}

.metric-value.compact {
  font-size: 18px;
}

.metric-value.profit {
  color: #16a34a;
}

.metric-value.loss {
  color: #dc2626;
}

.small-value {
  font-size: 18px;
}

.main-tabs {
  background: #fff;
  padding: 0 16px 16px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.data-table {
  width: 100%;
}

.pair-text {
  color: #374151;
  font-weight: 500;
}

.asset-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.text-up {
  color: #16a34a;
}

.text-down {
  color: #dc2626;
}

.strong {
  font-weight: 650;
}

@media (max-width: 900px) {
  .page-header {
    display: block;
  }

  .header-actions {
    justify-content: flex-start;
    margin-top: 12px;
  }
}
</style>
