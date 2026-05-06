<template>
  <div class="strategies">
    <h2>策略配置</h2>

    <el-row :gutter="16">
      <el-col :span="8" v-for="strategy in strategies" :key="strategy.name">
        <el-card class="strategy-card">
          <template #header>
            <div class="card-header">
              <span class="strategy-name">{{ strategy.displayName }}</span>
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

            <el-form-item>
              <el-button type="primary" @click="saveStrategy(strategy)" style="width: 100%">
                保存配置
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useStrategyStore, Strategy } from '@/stores/strategy'

const strategyStore = useStrategyStore()
const { strategies } = strategyStore

const toggleStrategy = async (strategy: Strategy) => {
  await strategyStore.toggleEnabled(strategy.name, strategy.isEnabled)
  ElMessage.success(`${strategy.displayName} 已${strategy.isEnabled ? '启用' : '禁用'}`)
}

const updateAutoExecute = async (strategy: Strategy) => {
  await strategyStore.setAutoExecute(strategy.name, strategy.autoExecute)
}

const saveStrategy = async (strategy: Strategy) => {
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
</style>
