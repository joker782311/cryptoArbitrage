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
  const tickers = ref<Ticker[]>([])
  const loading = ref(false)
  const wsConnected = ref(false)

  async function fetchTickers() {
    loading.value = true
    try {
      // TODO: 替换为实际 API
      tickers.value = [
        {
          exchange: 'binance',
          symbol: 'BTCUSDT',
          price: 67500,
          bid: 67499,
          ask: 67501,
          volume24h: 125000000,
          change24h: 2.5,
          high24h: 68000,
          low24h: 66000,
          timestamp: Date.now(),
        },
        {
          exchange: 'okx',
          symbol: 'BTCUSDT',
          price: 67520,
          bid: 67518,
          ask: 67522,
          volume24h: 98000000,
          change24h: 2.6,
          high24h: 68100,
          low24h: 66100,
          timestamp: Date.now(),
        },
        {
          exchange: 'bitget',
          symbol: 'BTCUSDT',
          price: 67480,
          bid: 67478,
          ask: 67482,
          volume24h: 45000000,
          change24h: 2.3,
          high24h: 67900,
          low24h: 65900,
          timestamp: Date.now(),
        },
        {
          exchange: 'binance',
          symbol: 'ETHUSDT',
          price: 3450,
          bid: 3449,
          ask: 3451,
          volume24h: 85000000,
          change24h: 3.2,
          high24h: 3500,
          low24h: 3350,
          timestamp: Date.now(),
        },
      ]
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
