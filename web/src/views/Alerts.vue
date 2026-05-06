<template>
  <div class="alerts">
    <h2>
      告警中心
      <el-badge :value="unreadCount" :hidden="unreadCount === 0">
        <el-button size="small" @click="markAllRead">全部已读</el-button>
      </el-badge>
    </h2>

    <el-row :gutter="16" style="margin-bottom: 20px;">
      <el-col :span="6">
        <el-statistic title="总告警数" :value="stats.total" />
      </el-col>
      <el-col :span="6">
        <el-statistic title="未读" :value="unreadCount" />
      </el-col>
      <el-col :span="6">
        <el-statistic title="今日" :value="stats.today" />
      </el-col>
    </el-row>

    <el-timeline>
      <el-timeline-item
        v-for="alert in alerts"
        :key="alert.id"
        :timestamp="alert.createdAt"
        placement="top"
        :type="getAlertType(alert.level)"
        :hollow="alert.isRead"
      >
        <el-card :class="{ 'is-read': alert.isRead }">
          <div class="alert-header">
            <el-tag :type="getAlertType(alert.level)" size="small">{{ alert.level }}</el-tag>
            <span class="alert-type">{{ getAlertTypeName(alert.type) }}</span>
          </div>
          <h4>{{ alert.title }}</h4>
          <p>{{ alert.message }}</p>
        </el-card>
      </el-timeline-item>
    </el-timeline>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAlertStore } from '@/stores/alert'

const alertStore = useAlertStore()
const { alerts, unreadCount, stats } = alertStore

const getAlertType = (level: string) => {
  const types: Record<string, any> = {
    info: 'info',
    warning: 'warning',
    error: 'danger',
    critical: 'danger',
  }
  return types[level] || 'info'
}

const getAlertTypeName = (type: string) => {
  const names: Record<string, string> = {
    opportunity: '套利机会',
    order: '订单',
    price: '价格',
    risk: '风控',
    system: '系统',
  }
  return names[type] || type
}

const markAllRead = () => {
  alertStore.markAllRead()
}

onMounted(() => {
  alertStore.fetchAlerts()
  alertStore.fetchStats()
})
</script>

<style scoped>
.alerts {
  padding: 20px;
}

h2 {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.alert-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.alert-type {
  font-size: 14px;
  color: #909399;
}

.is-read {
  opacity: 0.6;
}
</style>
