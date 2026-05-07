import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import api from '@/api'

export type SpotPerpDirection = 'spot_long_perp_short' | 'spot_short_inventory_perp_long'
export type MarketStatus = 'running' | 'halted'

export interface SimAccount {
  exchange: string
  usdt: number
  perpUsdt: number
  frozenUsdt: number
  spotBalances: Record<string, number>
  perpPositions: Record<string, number>
}

export interface SpotPerpOpportunity {
  id: string
  symbol: string
  direction: SpotPerpDirection
  spotExchange: string
  perpExchange: string
  spotPrice: number
  perpPrice: number
  notional: number
  basisAmount: number
  fundingAmount: number
  feeCost: number
  slippage: number
  safetyBuffer: number
  netProfit: number
  profitRate: number
  status: 'ready' | 'blocked'
  blockReason?: string
}

export interface SimPosition {
  id: string
  opportunityId: string
  symbol: string
  direction: SpotPerpDirection
  spotExchange: string
  perpExchange: string
  quantity: number
  notional: number
  margin: number
  spotPrice: number
  perpPrice: number
  netProfit: number
  realizedPnL: number
  openedAt: number
  closedAt?: number
  status: 'open' | 'closing' | 'closed'
}

export interface CloseAction {
  id: string
  positionId: string
  reason: string
  spotAction: string
  perpAction: string
  createdAt: number
}

interface SimulationSnapshot {
  status: MarketStatus
  haltReason: string
  lastQuoteAt: number
  marketErrors: Record<string, string>
  accounts: SimAccount[]
  opportunities: SpotPerpOpportunity[]
  positions: SimPosition[]
  closeActions: CloseAction[]
  pnl: {
    realizedPnL: number
    unrealizedPnL: number
    totalPnL: number
    openNotional: number
  }
}

export const useCexSpotPerpStore = defineStore('cexSpotPerp', () => {
  const loading = ref(false)
  const wsConnected = ref(false)
  const status = ref<MarketStatus>('running')
  const haltReason = ref('')
  const accounts = ref<SimAccount[]>([])
  const opportunities = ref<SpotPerpOpportunity[]>([])
  const positions = ref<SimPosition[]>([])
  const closeActions = ref<CloseAction[]>([])
  const lastQuoteAt = ref(0)
  const marketErrors = ref<Record<string, string>>({})
  const pnl = ref({
    realizedPnL: 0,
    unrealizedPnL: 0,
    totalPnL: 0,
    openNotional: 0,
  })

  const totalSpotUsdt = computed(() => accounts.value.reduce((sum, account) => sum + account.usdt, 0))
  const totalPerpUsdt = computed(() => accounts.value.reduce((sum, account) => sum + account.perpUsdt, 0))
  const totalFrozenUsdt = computed(() => accounts.value.reduce((sum, account) => sum + account.frozenUsdt, 0))
  const readyOpportunities = computed(() => opportunities.value.filter(opp => opp.status === 'ready'))
  const openPositions = computed(() => positions.value.filter(position => position.status === 'open'))
  let ws: WebSocket | null = null
  let reconnectTimer: number | undefined

  function hydrate(snapshot: SimulationSnapshot) {
    status.value = snapshot.status
    haltReason.value = snapshot.haltReason || ''
    accounts.value = snapshot.accounts || []
    opportunities.value = snapshot.opportunities || []
    positions.value = snapshot.positions || []
    closeActions.value = snapshot.closeActions || []
    lastQuoteAt.value = snapshot.lastQuoteAt || 0
    marketErrors.value = snapshot.marketErrors || {}
    pnl.value = snapshot.pnl || { realizedPnL: 0, unrealizedPnL: 0, totalPnL: 0, openNotional: 0 }
  }

  async function fetchSimulation() {
    loading.value = true
    try {
      const response = await api.get('/cex-spot-perp') as unknown as SimulationSnapshot
      hydrate(response)
    } finally {
      loading.value = false
    }
  }

  function connectWebSocket() {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return
    if (reconnectTimer) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = undefined
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${protocol}//${window.location.host}/api/v1/cex-spot-perp/ws`)

    ws.onopen = () => {
      wsConnected.value = true
    }
    ws.onmessage = (event) => {
      const message = JSON.parse(event.data) as { type: string; data: SimulationSnapshot }
      if (message.type === 'cex_spot_perp_snapshot') {
        hydrate(message.data)
      }
    }
    ws.onclose = () => {
      wsConnected.value = false
      ws = null
      reconnectTimer = window.setTimeout(connectWebSocket, 2000)
    }
    ws.onerror = () => {
      ws?.close()
    }
  }

  function disconnectWebSocket() {
    if (reconnectTimer) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = undefined
    }
    wsConnected.value = false
    ws?.close()
    ws = null
  }

  async function executeOpportunity(opp: SpotPerpOpportunity) {
    if (status.value === 'halted' || opp.status !== 'ready') return
    const response = await api.post(`/cex-spot-perp/opportunities/${encodeURIComponent(opp.id)}/execute`) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function closePosition(positionId: string, reason = '手动平仓') {
    const response = await api.post(`/cex-spot-perp/positions/${encodeURIComponent(positionId)}/close`, { reason }) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function triggerCircuitBreaker(reason: string) {
    const response = await api.post('/cex-spot-perp/circuit-breaker', { reason }) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function resumeSimulation() {
    const response = await api.post('/cex-spot-perp/resume') as unknown as SimulationSnapshot
    hydrate(response)
  }

  return {
    loading,
    wsConnected,
    status,
    haltReason,
    accounts,
    opportunities,
    positions,
    closeActions,
    lastQuoteAt,
    marketErrors,
    pnl,
    totalSpotUsdt,
    totalPerpUsdt,
    totalFrozenUsdt,
    readyOpportunities,
    openPositions,
    fetchSimulation,
    connectWebSocket,
    disconnectWebSocket,
    executeOpportunity,
    closePosition,
    triggerCircuitBreaker,
    resumeSimulation,
  }
})
