import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

export interface Position {
  exchange: string
  symbol: string
  side: string
  quantity: number
  entryPrice: number
  currentPrice: number
  pnl: number
  pnlPercent: number
}

export const usePositionStore = defineStore('position', () => {
  const positions = ref<Position[]>([])
  const stats = ref({
    totalValue: 0,
    totalPnl: 0,
    totalPnlPercent: 0,
    longPositions: 0,
    shortPositions: 0,
  })
  const loading = ref(false)

  async function fetchPositions() {
    loading.value = true
    try {
      // TODO: 替换为实际 API
      positions.value = [
        {
          exchange: 'binance',
          symbol: 'BTCUSDT',
          side: 'long',
          quantity: 0.5,
          entryPrice: 65000,
          currentPrice: 67500,
          pnl: 1250,
          pnlPercent: 3.85,
        },
        {
          exchange: 'okx',
          symbol: 'ETHUSDT',
          side: 'short',
          quantity: 5,
          entryPrice: 3500,
          currentPrice: 3450,
          pnl: 250,
          pnlPercent: 1.43,
        },
      ]

      updateStats()
    } finally {
      loading.value = false
    }
  }

  async function fetchStats() {
    // TODO: API 调用
    stats.value = {
      totalValue: 50000,
      totalPnl: 1500,
      totalPnlPercent: 3,
      longPositions: 1,
      shortPositions: 1,
    }
  }

  function updateStats() {
    stats.value.totalValue = positions.value.reduce(
      (sum, p) => sum + p.currentPrice * p.quantity,
      0
    )
    stats.value.totalPnl = positions.value.reduce((sum, p) => sum + p.pnl, 0)
    stats.value.longPositions = positions.value.filter(p => p.side === 'long').length
    stats.value.shortPositions = positions.value.filter(p => p.side === 'short').length
  }

  return { positions, stats, loading, fetchPositions, fetchStats }
})
