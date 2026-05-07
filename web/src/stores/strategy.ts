import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

export interface Strategy {
  name: string
  displayName: string
  isEnabled: boolean
  autoExecute: boolean
  minProfitRate: number
  maxPosition: number
  stopLossRate: number
  config: Record<string, unknown>
}

export const useStrategyStore = defineStore('strategy', () => {
  const strategies = ref<Strategy[]>([])
  const loading = ref(false)

  async function fetchStrategies() {
    loading.value = true
    try {
      // TODO: 替换为实际 API
      strategies.value = [
        {
          name: 'cex_spot_perp',
          displayName: '跨所期现模拟盘',
          isEnabled: true,
          autoExecute: true,
          minProfitRate: 0.2,
          maxPosition: 3000,
          stopLossRate: 2,
          config: {
            directions: ['spot_long_perp_short', 'spot_short_inventory_perp_long'],
            exchanges: ['binance', 'okx', 'bitget'],
            symbols: ['BTCUSDT', 'ETHUSDT', 'SOLUSDT'],
          },
        },
        {
          name: 'cross_exchange',
          displayName: '跨交易所套利',
          isEnabled: true,
          autoExecute: true,
          minProfitRate: 0.5,
          maxPosition: 10000,
          stopLossRate: 2,
          config: {},
        },
        {
          name: 'funding_rate',
          displayName: '资金费率套利',
          isEnabled: true,
          autoExecute: false,
          minProfitRate: 1,
          maxPosition: 20000,
          stopLossRate: 3,
          config: {},
        },
        {
          name: 'spot_future',
          displayName: '期现套利',
          isEnabled: false,
          autoExecute: false,
          minProfitRate: 0.3,
          maxPosition: 15000,
          stopLossRate: 2,
          config: {},
        },
        {
          name: 'triangular',
          displayName: '三角套利',
          isEnabled: true,
          autoExecute: true,
          minProfitRate: 0.2,
          maxPosition: 5000,
          stopLossRate: 1,
          config: {},
        },
        {
          name: 'dex_cross_dex',
          displayName: '跨 DEX 套利',
          isEnabled: false,
          autoExecute: false,
          minProfitRate: 1,
          maxPosition: 3000,
          stopLossRate: 5,
          config: {},
        },
      ]
    } finally {
      loading.value = false
    }
  }

  async function toggleEnabled(name: string, enabled: boolean) {
    // TODO: API 调用
    const strategy = strategies.value.find(s => s.name === name)
    if (strategy) {
      strategy.isEnabled = enabled
    }
  }

  async function setAutoExecute(name: string, auto: boolean) {
    const strategy = strategies.value.find(s => s.name === name)
    if (strategy) {
      strategy.autoExecute = auto
    }
  }

  async function updateStrategy(strategy: Strategy) {
    // TODO: API 调用
    const index = strategies.value.findIndex(s => s.name === strategy.name)
    if (index >= 0) {
      strategies.value[index] = strategy
    }
  }

  return { strategies, loading, fetchStrategies, toggleEnabled, setAutoExecute, updateStrategy }
})
