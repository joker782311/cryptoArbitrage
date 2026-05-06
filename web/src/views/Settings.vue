<template>
  <div class="settings">
    <h2>系统设置</h2>

    <el-tabs>
      <el-tab-pane label="交易所配置">
        <el-card style="margin-bottom: 16px;" v-for="ex in exchanges" :key="ex.name">
          <template #header>
            <div class="card-header">
              <span>{{ ex.displayName }}</span>
              <el-switch v-model="ex.enabled" />
            </div>
          </template>
          <el-form label-width="120px" size="large">
            <el-form-item label="API Key">
              <el-input v-model="ex.apiKey" placeholder="请输入 API Key" />
            </el-form-item>
            <el-form-item label="API Secret">
              <el-input v-model="ex.secret" type="password" placeholder="请输入 API Secret" />
            </el-form-item>
            <el-form-item label="Passphrase" v-if="ex.needPassphrase">
              <el-input v-model="ex.passphrase" placeholder="请输入 Passphrase" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveApiKey(ex)">保存配置</el-button>
              <el-button type="danger" @click="deleteApiKey(ex.name)">删除</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="告警配置">
        <el-button type="primary" @click="showAddAlertConfig = true" style="margin-bottom: 16px;">
          添加告警渠道
        </el-button>
        <el-table :data="alertConfigs" stripe>
          <el-table-column prop="channel" label="渠道" width="120">
            <template #default="{ row }">
              <el-tag>{{ row.channel }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="webhookUrl" label="Webhook URL" />
          <el-table-column prop="email" label="邮箱" />
          <el-table-column prop="chatId" label="Chat ID" />
          <el-table-column prop="isEnabled" label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.isEnabled ? 'success' : 'info'">
                {{ row.isEnabled ? '启用' : '禁用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150">
            <template #default="{ row }">
              <el-button size="small" @click="editAlertConfig(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteAlertConfig(row.id)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="风控配置">
        <el-form label-width="150px" style="max-width: 500px;">
          <el-form-item label="最大总仓位 (USDT)">
            <el-input-number v-model="riskConfig.maxPosition" :step="1000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="日止损限额 (USDT)">
            <el-input-number v-model="riskConfig.dailyStopLoss" :step="100" :step-strictly="false" style="width: 100%" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="saveRiskConfig">保存配置</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const exchanges = ref([
  { name: 'binance', displayName: '币安', enabled: true, apiKey: '', secret: '', needPassphrase: false },
  { name: 'okx', displayName: 'OKX', enabled: true, apiKey: '', secret: '', passphrase: '', needPassphrase: true },
  { name: 'bitget', displayName: 'Bitget', enabled: false, apiKey: '', secret: '', passphrase: '', needPassphrase: true },
])

const alertConfigs = ref([
  { id: 1, channel: 'telegram', webhookUrl: '', email: '', chatId: '123456', isEnabled: true },
  { id: 2, channel: 'slack', webhookUrl: 'https://hooks.slack.com/xxx', email: '', chatId: '', isEnabled: false },
])

const riskConfig = ref({
  maxPosition: 100000,
  dailyStopLoss: 5000,
})

const showAddAlertConfig = ref(false)

const saveApiKey = (ex: any) => {
  ElMessage.success(`${ex.displayName} API Key 已保存`)
}

const deleteApiKey = (name: string) => {
  ElMessage.success(`${name} API Key 已删除`)
}

const editAlertConfig = (config: any) => {
  ElMessage.info('编辑告警配置')
}

const deleteAlertConfig = (id: number) => {
  ElMessage.success('告警配置已删除')
}

const saveRiskConfig = () => {
  ElMessage.success('风控配置已保存')
}
</script>

<style scoped>
.settings {
  padding: 20px;
}

h2 {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
