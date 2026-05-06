<template>
  <div class="positions">
    <h2>仓位管理</h2>

    <el-row :gutter="16" style="margin-bottom: 20px;">
      <el-col :span="6">
        <el-statistic title="总仓位价值" :value="stats.totalValue">
          <template #suffix>
            <span style="font-size: 14px;">USDT</span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="6">
        <el-statistic title="总盈亏" :value="stats.totalPnl">
          <template #suffix>
            <span style="font-size: 14px;">USDT</span>
          </template>
          <template v-slot:formatter="props">
            <span :class="props.value >= 0 ? 'text-up' : 'text-down'">
              {{ props.value >= 0 ? '+' : '' }}{{ props.value.toFixed(2) }}
            </span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="6">
        <el-statistic title="收益率" :value="stats.totalPnlPercent" :precision="2">
          <template #suffix>%</template>
          <template v-slot:formatter="props">
            <span :class="props.value >= 0 ? 'text-up' : 'text-down'">
              {{ props.value >= 0 ? '+' : '' }}{{ props.value.toFixed(2) }}
            </span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="6">
        <el-descriptions :column="2" size="small">
          <el-descriptions-item label="多头">{{ stats.longPositions }}</el-descriptions-item>
          <el-descriptions-item label="空头">{{ stats.shortPositions }}</el-descriptions-item>
        </el-descriptions>
      </el-col>
    </el-row>

    <el-table :data="positions" stripe v-loading="loading">
      <el-table-column prop="exchange" label="交易所" width="100">
        <template #default="{ row }">
          <el-tag :type="getExchangeType(row.exchange)">{{ row.exchange }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="symbol" label="交易对" width="120" />
      <el-table-column prop="side" label="方向" width="80">
        <template #default="{ row }">
          <el-tag :type="row.side === 'long' ? 'success' : 'danger'">
            {{ row.side === 'long' ? '多' : '空' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="quantity" label="数量" width="120" />
      <el-table-column prop="entryPrice" label="开仓价" width="120">
        <template #default="{ row }">
          ${{ row.entryPrice.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
        </template>
      </el-table-column>
      <el-table-column prop="currentPrice" label="当前价" width="120">
        <template #default="{ row }">
          ${{ row.currentPrice.toLocaleString(undefined, { minimumFractionDigits: 2 }) }}
        </template>
      </el-table-column>
      <el-table-column prop="pnl" label="盈亏" width="120">
        <template #default="{ row }">
          <span :class="row.pnl >= 0 ? 'text-up' : 'text-down'">
            {{ row.pnl >= 0 ? '+' : '' }}${{ row.pnl.toFixed(2) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="pnlPercent" label="收益率" width="100">
        <template #default="{ row }">
          <span :class="row.pnlPercent >= 0 ? 'text-up' : 'text-down'">
            {{ row.pnlPercent >= 0 ? '+' : '' }}{{ row.pnlPercent.toFixed(2) }}%
          </span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { usePositionStore } from '@/stores/position'

const positionStore = usePositionStore()
const { positions, stats, loading } = positionStore

const getExchangeType = (exchange: string) => {
  const types: Record<string, any> = {
    binance: 'warning',
    okx: 'success',
    bitget: 'info',
  }
  return types[exchange] || ''
}

onMounted(() => {
  positionStore.fetchPositions()
})
</script>

<style scoped>
.positions {
  padding: 20px;
}

h2 {
  margin-bottom: 20px;
}

.text-up {
  color: #67c23a;
}

.text-down {
  color: #f56c6c;
}
</style>
