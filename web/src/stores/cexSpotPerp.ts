import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import api from '@/api'

export type SpotPerpDirection = 'spot_long_perp_short' | 'spot_short_inventory_perp_long'
export type MarketStatus = 'running' | 'halted'

export interface SpotPerpConfig {
  symbols: string[]
  exchanges: string[]
  exchangeSymbols: Record<string, string[]>
  leverage: number
  maxLeverage: number
  minNetProfitRate: number
  carryFundingIntervals: number
}

export interface SimAccount {
  exchange: string
  usdt: number
  perpUsdt: number
  frozenUsdt: number
  spotBalances: Record<string, number>
  perpPositions: Record<string, number>
}

export interface MarketQuote {
  exchange: string
  symbol: string
  marketType: 'spot' | 'perp'
  bid: number
  ask: number
  last: number
  fundingRate: number
  timestamp: number
  ageMillis: number
  stale: boolean
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
  carryFundingAmount: number
  carryNetProfit: number
  carryProfitRate: number
  carryFundingIntervals: number
  status: 'ready' | 'watch' | 'blocked'
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

export interface SpotPerpAutomation {
  enabled: boolean
  autoOpen: boolean
  autoClose: boolean
  openMinProfitRate: number
  closeMinProfitRate: number
  maxHoldSeconds: number
  maxOpenPositions: number
  checkIntervalMillis: number
}

export interface SpotPerpAutoStats {
  autoOpenCount: number
  autoCloseCount: number
  winCount: number
  lossCount: number
  totalProfit: number
  averageProfit: number
  winRate: number
  lastActionAt: number
  lastActionError: string
}

export interface OpportunityLog {
  key: string
  id: string
  symbol: string
  direction: SpotPerpDirection
  spotExchange: string
  perpExchange: string
  firstSeenAt: number
  lastSeenAt: number
  seenCount: number
  bestProfit: number
  bestProfitRate: number
  lastProfit: number
  lastProfitRate: number
  lastStatus: 'ready' | 'watch' | 'blocked'
  lastBlockReason: string
  autoOpenedCount: number
  autoRejectedNote: string
}

export interface AutoTrade {
  id: string
  positionId: string
  opportunity: string
  symbol: string
  direction: SpotPerpDirection
  spotExchange: string
  perpExchange: string
  action: 'open' | 'close'
  reason: string
  quantity: number
  notional: number
  margin: number
  spotValue: number
  capitalUsed: number
  profit: number
  profitRate: number
  createdAt: number
}

const defaultTopSymbols = [
  'BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'SOLUSDT', 'XRPUSDT',
  'DOGEUSDT', 'ADAUSDT', 'TRXUSDT', 'LINKUSDT', 'AVAXUSDT',
  'TONUSDT', 'SHIBUSDT', 'DOTUSDT', 'BCHUSDT', 'LTCUSDT',
  'UNIUSDT', 'NEARUSDT', 'APTUSDT', 'ICPUSDT', 'ETCUSDT',
]

interface SimulationSnapshot {
  status: MarketStatus
  haltReason: string
  lastQuoteAt: number
  marketErrors: Record<string, string>
  wsStatus: Record<string, string>
  wsErrors: Record<string, string>
  config: SpotPerpConfig
  accounts: SimAccount[]
  quotes: MarketQuote[]
  opportunities: SpotPerpOpportunity[]
  positions: SimPosition[]
  closeActions: CloseAction[]
  opportunityLogs: OpportunityLog[]
  autoTrades: AutoTrade[]
  automation: SpotPerpAutomation
  autoStats: SpotPerpAutoStats
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
  const config = ref<SpotPerpConfig>({
    symbols: [...defaultTopSymbols],
    exchanges: ['binance', 'okx', 'bitget'],
    exchangeSymbols: {
      binance: [...defaultTopSymbols],
      okx: [...defaultTopSymbols],
      bitget: [...defaultTopSymbols],
    },
    leverage: 3,
    maxLeverage: 3,
    minNetProfitRate: 0.05,
    carryFundingIntervals: 6,
  })
  const accounts = ref<SimAccount[]>([])
  const quotes = ref<MarketQuote[]>([])
  const opportunities = ref<SpotPerpOpportunity[]>([])
  const positions = ref<SimPosition[]>([])
  const closeActions = ref<CloseAction[]>([])
  const opportunityLogs = ref<OpportunityLog[]>([])
  const autoTrades = ref<AutoTrade[]>([])
  const automation = ref<SpotPerpAutomation>({
    enabled: false,
    autoOpen: true,
    autoClose: true,
    openMinProfitRate: 0.05,
    closeMinProfitRate: 0,
    maxHoldSeconds: 300,
    maxOpenPositions: 3,
    checkIntervalMillis: 1000,
  })
  const autoStats = ref<SpotPerpAutoStats>({
    autoOpenCount: 0,
    autoCloseCount: 0,
    winCount: 0,
    lossCount: 0,
    totalProfit: 0,
    averageProfit: 0,
    winRate: 0,
    lastActionAt: 0,
    lastActionError: '',
  })
  const lastQuoteAt = ref(0)
  const marketErrors = ref<Record<string, string>>({})
  const wsStatus = ref<Record<string, string>>({})
  const wsErrors = ref<Record<string, string>>({})
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
  const watchOpportunities = computed(() => opportunities.value.filter(opp => opp.status === 'watch'))
  const openPositions = computed(() => positions.value.filter(position => position.status === 'open'))
  let ws: WebSocket | null = null
  let reconnectTimer: number | undefined

  function hydrate(snapshot: SimulationSnapshot) {
    status.value = snapshot.status
    haltReason.value = snapshot.haltReason || ''
    config.value = snapshot.config || config.value
    accounts.value = snapshot.accounts || []
    quotes.value = snapshot.quotes || []
    opportunities.value = snapshot.opportunities || []
    positions.value = snapshot.positions || []
    closeActions.value = snapshot.closeActions || []
    opportunityLogs.value = snapshot.opportunityLogs || []
    autoTrades.value = snapshot.autoTrades || []
    automation.value = snapshot.automation || automation.value
    autoStats.value = snapshot.autoStats || autoStats.value
    lastQuoteAt.value = snapshot.lastQuoteAt || 0
    marketErrors.value = snapshot.marketErrors || {}
    wsStatus.value = snapshot.wsStatus || {}
    wsErrors.value = snapshot.wsErrors || {}
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

  async function updateConfig(nextConfig: SpotPerpConfig) {
    const response = await api.put('/cex-spot-perp/config', nextConfig) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function updateAutomation(nextAutomation: SpotPerpAutomation) {
    const response = await api.put('/cex-spot-perp/automation', nextAutomation) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function updateAccount(account: SimAccount) {
    const response = await api.put(`/cex-spot-perp/accounts/${encodeURIComponent(account.exchange)}`, {
      usdt: account.usdt,
      perpUsdt: account.perpUsdt,
      spotBalances: account.spotBalances,
    }) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function transferAccountUSDT(exchange: string, from: 'spot' | 'perp', to: 'spot' | 'perp', amount: number) {
    const response = await api.post(`/cex-spot-perp/accounts/${encodeURIComponent(exchange)}/transfer`, {
      from,
      to,
      amount,
    }) as unknown as SimulationSnapshot
    hydrate(response)
  }

  async function resetAccounts() {
    const response = await api.post('/cex-spot-perp/accounts/reset') as unknown as SimulationSnapshot
    hydrate(response)
  }

  return {
    loading,
    wsConnected,
    status,
    haltReason,
    config,
    accounts,
    quotes,
    opportunities,
    positions,
    closeActions,
    opportunityLogs,
    autoTrades,
    automation,
    autoStats,
    lastQuoteAt,
    marketErrors,
    wsStatus,
    wsErrors,
    pnl,
    totalSpotUsdt,
    totalPerpUsdt,
    totalFrozenUsdt,
    readyOpportunities,
    watchOpportunities,
    openPositions,
    fetchSimulation,
    connectWebSocket,
    disconnectWebSocket,
    executeOpportunity,
    closePosition,
    triggerCircuitBreaker,
    resumeSimulation,
    updateConfig,
    updateAutomation,
    updateAccount,
    transferAccountUSDT,
    resetAccounts,
  }
})
