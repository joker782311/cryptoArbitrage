<template>
  <div class="strategies">
    <h2>策略配置</h2>

    <el-row :gutter="16">
      <el-col :xs="24" :md="12" :xl="8" v-for="strategy in strategies" :key="strategy.name">
        <el-card class="strategy-card">
          <template #header>
            <div class="card-header">
              <div>
                <span class="strategy-name">{{ strategy.displayName }}</span>
                <el-tag v-if="strategy.name === 'cex_spot_perp'" class="strategy-tag" type="success" size="small">
                  模拟盘
                </el-tag>
              </div>
              <el-switch
                v-model="strategy.isEnabled"
                @change="toggleStrategy(strategy)"
              />
            </div>
          </template>

          <el-form label-position="top" size="small">
            <el-form-item label="自动执行">
              <el-switch
                v-model="strategy.autoExecute"
                :disabled="!strategy.isEnabled"
                @change="updateAutoExecute(strategy)"
              />
            </el-form-item>

            <el-form-item label="最小利润率 (%)">
              <el-input-number
                v-model="strategy.minProfitRate"
                :min="0"
                :max="10"
                :step="0.1"
                :precision="2"
                style="width: 100%"
              />
            </el-form-item>

            <el-form-item label="最大仓位 (USDT)">
              <el-input-number
                v-model="strategy.maxPosition"
                :min="0"
                :step="1000"
                style="width: 100%"
              />
            </el-form-item>

            <el-form-item label="止损率 (%)">
              <el-input-number
                v-model="strategy.stopLossRate"
                :min="0"
                :max="100"
                :step="0.5"
                :precision="2"
                style="width: 100%"
              />
            </el-form-item>

            <template v-if="strategy.name === 'cex_spot_perp'">
              <el-form-item label="交易所组合">
                <el-checkbox-group v-model="spotPerpExchanges">
                  <el-checkbox-button label="binance">Binance</el-checkbox-button>
                  <el-checkbox-button label="okx">OKX</el-checkbox-button>
                  <el-checkbox-button label="bitget">Bitget</el-checkbox-button>
                </el-checkbox-group>
              </el-form-item>

              <el-form-item label="交易对白名单">
                <el-select v-model="spotPerpSymbols" multiple filterable allow-create default-first-option style="width: 100%">
                  <el-option label="BTCUSDT" value="BTCUSDT" />
                  <el-option label="ETHUSDT" value="ETHUSDT" />
                  <el-option label="SOLUSDT" value="SOLUSDT" />
                </el-select>
              </el-form-item>

              <el-form-item label="策略方向">
                <div class="direction-tags">
                  <el-tag type="success">买现货 + 空永续</el-tag>
                  <el-tag type="warning">卖库存 + 多永续</el-tag>
                </div>
              </el-form-item>
            </template>

            <el-form-item>
              <el-button type="primary" @click="saveStrategy(strategy)" style="width: 100%">
                保存配置
              </el-button>
              <el-button
                v-if="strategy.name === 'cex_spot_perp'"
                class="open-sim-button"
                @click="$router.push('/cex-spot-perp')"
                style="width: 100%"
              >
                打开模拟盘
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useStrategyStore } from '@/stores/strategy'
import type { Strategy } from '@/stores/strategy'

const strategyStore = useStrategyStore()
const { strategies } = strategyStore
const spotPerpExchanges = ref(['binance', 'okx', 'bitget'])
const spotPerpSymbols = ref(['BTCUSDT', 'ETHUSDT', 'SOLUSDT'])

const toggleStrategy = async (strategy: Strategy) => {
  await strategyStore.toggleEnabled(strategy.name, strategy.isEnabled)
  ElMessage.success(`${strategy.displayName} 已${strategy.isEnabled ? '启用' : '禁用'}`)
}

const updateAutoExecute = async (strategy: Strategy) => {
  await strategyStore.setAutoExecute(strategy.name, strategy.autoExecute)
}

const saveStrategy = async (strategy: Strategy) => {
  if (strategy.name === 'cex_spot_perp') {
    strategy.config = {
      ...strategy.config,
      exchanges: spotPerpExchanges.value,
      symbols: spotPerpSymbols.value,
    }
  }
  await strategyStore.updateStrategy(strategy)
  ElMessage.success('配置已保存')
}

onMounted(() => {
  strategyStore.fetchStrategies()
})
</script>

<style scoped>
.strategies {
  padding: 20px;
}

h2 {
  margin-bottom: 20px;
}

.strategy-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.strategy-name {
  font-weight: bold;
  font-size: 16px;
}

.strategy-tag {
  margin-left: 8px;
}

.direction-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.open-sim-button {
  margin-top: 8px;
  margin-left: 0;
}
</style>
