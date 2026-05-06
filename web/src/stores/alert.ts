import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

export interface Alert {
  id: number
  type: string
  level: string
  title: string
  message: string
  isRead: boolean
  createdAt: string
}

export const useAlertStore = defineStore('alert', () => {
  const alerts = ref<Alert[]>([])
  const unreadCount = ref(0)
  const stats = ref({
    total: 0,
    unread: 0,
    today: 0,
    byType: {} as Record<string, number>,
    byLevel: {} as Record<string, number>,
  })
  const loading = ref(false)

  async function fetchAlerts(limit = 50) {
    loading.value = true
    try {
      // TODO: 替换为实际 API
      alerts.value = [
        {
          id: 1,
          type: 'opportunity',
          level: 'info',
          title: '发现套利机会',
          message: '跨交易所套利：BTCUSDT 利润率 0.6%',
          isRead: false,
          createdAt: new Date().toISOString(),
        },
        {
          id: 2,
          type: 'order',
          level: 'info',
          title: '订单成交',
          message: 'BTCUSDT 买入订单已成交',
          isRead: false,
          createdAt: new Date().toISOString(),
        },
        {
          id: 3,
          type: 'risk',
          level: 'warning',
          title: '风控触发',
          message: '日亏损接近限制',
          isRead: true,
          createdAt: new Date().toISOString(),
        },
      ]

      unreadCount.value = alerts.value.filter(a => !a.isRead).length
    } finally {
      loading.value = false
    }
  }

  async function fetchStats() {
    stats.value = {
      total: 100,
      unread: 5,
      today: 20,
      byType: { opportunity: 50, order: 30, risk: 20 },
      byLevel: { info: 70, warning: 25, error: 5 },
    }
  }

  async function markAsRead(ids: number[]) {
    // TODO: API 调用
    alerts.value.forEach(alert => {
      if (ids.includes(alert.id)) {
        alert.isRead = true
      }
    })
    unreadCount.value = alerts.value.filter(a => !a.isRead).length
  }

  async function markAllRead() {
    alerts.value.forEach(a => a.isRead = true)
    unreadCount.value = 0
  }

  return { alerts, unreadCount, stats, loading, fetchAlerts, fetchStats, markAsRead, markAllRead }
})
