import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

export interface Ticker {
  exchange: string
  symbol: string
  price: number
  bid: number
  ask: number
  volume24h: number
  change24h: number
  high24h: number
  low24h: number
  timestamp: number
}

export const useTickerStore = defineStore('ticker', () => {
  const tickers = ref<Ticker[]>([])  // 初始值为空数组
  const loading = ref(false)
  const wsConnected = ref(false)

  async function fetchTickers() {
    loading.value = true
    try {
      const response = await api.get('/tickers')
      console.log('Tickers response:', response)
      // axios interceptor 已返回 response.data
      // API 返回格式：{ tickers: [...] }
      if (response && response.tickers && Array.isArray(response.tickers)) {
        tickers.value = response.tickers
        console.log('Tickers loaded:', tickers.value.length)
      } else if (response && Array.isArray(response)) {
        tickers.value = response
        console.log('Tickers loaded (direct):', tickers.value.length)
      } else {
        console.warn('No tickers data received, response type:', typeof response)
      }
    } catch (error) {
      console.error('Failed to fetch tickers:', error)
    } finally {
      loading.value = false
    }
  }

  function connectWebSocket() {
    const ws = new WebSocket(`ws://localhost:8080/api/v1/ws`)

    ws.onopen = () => {
      wsConnected.value = true
    }

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data)
      if (data.type === 'ticker') {
        const index = tickers.value.findIndex(
          t => t.exchange === data.exchange && t.symbol === data.symbol
        )
        if (index >= 0) {
          tickers.value[index] = { ...tickers.value[index], ...data }
        } else {
          tickers.value.push(data)
        }
      }
    }

    ws.onclose = () => {
      wsConnected.value = false
      setTimeout(connectWebSocket, 5000)
    }
  }

  return { tickers, loading, wsConnected, fetchTickers, connectWebSocket }
})
