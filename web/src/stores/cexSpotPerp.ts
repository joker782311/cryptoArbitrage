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
  const status = ref<MarketStatus>('running')
  const haltReason = ref('')
  const accounts = ref<SimAccount[]>([])
  const opportunities = ref<SpotPerpOpportunity[]>([])
  const positions = ref<SimPosition[]>([])
  const closeActions = ref<CloseAction[]>([])
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

  function hydrate(snapshot: SimulationSnapshot) {
    status.value = snapshot.status
    haltReason.value = snapshot.haltReason || ''
    accounts.value = snapshot.accounts || []
    opportunities.value = snapshot.opportunities || []
    positions.value = snapshot.positions || []
    closeActions.value = snapshot.closeActions || []
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
    status,
    haltReason,
    accounts,
    opportunities,
    positions,
    closeActions,
    pnl,
    totalSpotUsdt,
    totalPerpUsdt,
    totalFrozenUsdt,
    readyOpportunities,
    openPositions,
    fetchSimulation,
    executeOpportunity,
    closePosition,
    triggerCircuitBreaker,
    resumeSimulation,
  }
})
