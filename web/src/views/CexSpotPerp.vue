<template>
  <div class="spot-perp-page">
    <div class="page-header">
      <div>
        <h2>跨所期现模拟盘</h2>
        <div class="subtitle">
          Binance / OKX / Bitget · 现货与 U 本位永续组合扫描 · 行情更新时间 {{ formatTs(lastQuoteAt) }}
        </div>
      </div>
      <div class="header-actions">
        <el-tag :type="status === 'running' ? 'success' : 'danger'" size="large">
          {{ status === 'running' ? '运行中' : '已熔断' }}
        </el-tag>
        <el-tag :type="wsConnected ? 'success' : 'warning'" size="large">
          {{ wsConnected ? 'WebSocket 实时' : 'WebSocket 重连中' }}
        </el-tag>
        <el-tag :type="marketWSConnectedCount > 0 ? 'success' : 'warning'" size="large">
          官方行情 {{ marketWSConnectedCount }}/{{ marketWSTotalCount }}
        </el-tag>
        <el-button :icon="Refresh" @click="store.fetchSimulation()">刷新</el-button>
        <el-button v-if="status === 'running'" type="danger" :icon="SwitchButton" @click="handleHalt">
          熔断清仓
        </el-button>
        <el-button v-else type="primary" :icon="VideoPlay" @click="store.resumeSimulation()">
          恢复模拟
        </el-button>
      </div>
    </div>

    <el-alert
      v-if="status === 'halted'"
      class="halt-alert"
      type="error"
      :closable="false"
      show-icon
      :title="`熔断原因：${haltReason}`"
    />
    <el-alert
      v-if="Object.keys(marketErrors).length > 0"
      class="halt-alert"
      type="warning"
      :closable="false"
      show-icon
      :title="`部分资金费率更新失败：${Object.keys(marketErrors).length} 项`"
    />
    <el-alert
      v-if="Object.keys(wsErrors).length > 0"
      class="halt-alert"
      type="warning"
      :closable="false"
      show-icon
      :title="`官方交易所 WebSocket 连接失败：${Object.keys(wsErrors).length} 路`"
    />

    <el-card shadow="never" class="config-panel">
      <el-form label-width="110px" class="config-form">
        <el-form-item label="合约杠杆">
          <div class="leverage-control">
            <el-slider v-model="draftConfig.leverage" :min="1" :max="3" :step="0.1" />
            <el-input-number v-model="draftConfig.leverage" :min="1" :max="3" :step="0.1" :precision="1" />
          </div>
        </el-form-item>
        <el-form-item label="交易所">
          <el-checkbox-group v-model="draftConfig.exchanges">
            <el-checkbox-button v-for="exchange in allExchanges" :key="exchange" :label="exchange">
              {{ exchange }}
            </el-checkbox-button>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="币种白名单">
          <div class="symbol-picker">
            <el-select
              v-model="draftConfig.symbols"
              multiple
              filterable
              allow-create
              default-first-option
              class="wide-select"
              placeholder="输入交易对并回车新增，例如 BTCUSDT"
            >
              <el-option v-for="symbol in commonSymbols" :key="symbol" :label="symbol" :value="symbol" />
            </el-select>
            <el-button @click="applyTopSymbols">填入 Top20</el-button>
          </div>
        </el-form-item>
        <el-form-item label="各所币种">
          <div class="exchange-symbols">
            <div v-for="exchange in draftConfig.exchanges" :key="exchange" class="exchange-symbol-row">
              <span class="exchange-label">{{ exchange }}</span>
              <el-select v-model="draftConfig.exchangeSymbols[exchange]" multiple filterable class="exchange-symbol-select">
                <el-option v-for="symbol in draftConfig.symbols" :key="`${exchange}-${symbol}`" :label="symbol" :value="symbol" />
              </el-select>
            </div>
          </div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Setting" @click="saveConfig">保存配置</el-button>
          <el-button v-if="hasUnsavedConfig" @click="resetDraftConfig">放弃修改</el-button>
          <el-tag v-if="hasUnsavedConfig" type="warning" effect="plain">有未保存配置</el-tag>
          <span class="muted">杠杆上限 3 倍，配置保存后会重新订阅行情源。</span>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="config-panel">
      <el-form label-width="130px" class="config-form">
        <el-form-item label="自动开关仓">
          <div class="automation-row">
            <el-switch v-model="automationDraft.enabled" active-text="启用" inactive-text="关闭" />
            <el-checkbox v-model="automationDraft.autoOpen">自动开仓</el-checkbox>
            <el-checkbox v-model="automationDraft.autoClose">自动平仓</el-checkbox>
            <el-button type="primary" @click="saveAutomation">保存自动化</el-button>
            <el-button v-if="hasUnsavedAutomation" @click="resetAutomationDraft">放弃修改</el-button>
            <el-tag v-if="hasUnsavedAutomation" type="warning" effect="plain">有未保存自动化</el-tag>
            <span class="muted">自动化关闭时，仍可在机会扫描和持仓里手动开仓 / 平仓。</span>
          </div>
        </el-form-item>
        <el-form-item label="开仓收益率">
          <el-input-number v-model="automationDraft.openMinProfitRate" :min="0" :precision="3" :step="0.01" />
          <span class="muted">机会收益率达到该阈值才允许自动开仓。</span>
        </el-form-item>
        <el-form-item label="平仓条件">
          <div class="automation-row">
            <span class="muted">预计收益率</span>
            <el-input-number v-model="automationDraft.closeMinProfitRate" :min="0" :precision="3" :step="0.01" />
            <span class="muted">最大持仓秒数</span>
            <el-input-number v-model="automationDraft.maxHoldSeconds" :min="0" :precision="0" :step="30" />
            <span class="muted">0 表示关闭该条件。</span>
          </div>
        </el-form-item>
        <el-form-item label="仓位上限">
          <div class="automation-row">
            <el-input-number v-model="automationDraft.maxOpenPositions" :min="1" :max="20" :precision="0" />
            <span class="muted">同一组合不会重复自动开仓。</span>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <el-row :gutter="16" class="summary-row">
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">累计净收益</div>
          <div :class="pnl.totalPnL >= 0 ? 'metric-value profit' : 'metric-value loss'">
            {{ signedMoney(pnl.totalPnL) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">已实现收益</div>
          <div :class="pnl.realizedPnL >= 0 ? 'metric-value profit' : 'metric-value loss'">
            {{ signedMoney(pnl.realizedPnL) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">未实现预估</div>
          <div :class="pnl.unrealizedPnL >= 0 ? 'metric-value profit' : 'metric-value loss'">
            {{ signedMoney(pnl.unrealizedPnL) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-label">持仓本金 / 冻结保证金</div>
          <div class="metric-value compact">{{ money(pnl.openNotional) }} / {{ money(totalFrozenUsdt) }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="summary-row secondary">
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">可用现货 USDT</div>
          <div class="metric-value small-value">{{ money(totalSpotUsdt) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">合约保证金账户</div>
          <div class="metric-value small-value">{{ money(totalPerpUsdt) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">可执行 / 观察机会</div>
          <div class="metric-value small-value">{{ readyOpportunities.length }} / {{ watchOpportunities.length }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="summary-row secondary">
      <el-col :xs="24" :sm="6">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">自动开仓 / 平仓</div>
          <div class="metric-value small-value">{{ autoStats.autoOpenCount }} / {{ autoStats.autoCloseCount }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="6">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">自动胜率</div>
          <div class="metric-value small-value">{{ autoStats.winRate.toFixed(2) }}%</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="6">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">自动累计利润</div>
          <div :class="autoStats.totalProfit >= 0 ? 'metric-value small-value profit' : 'metric-value small-value loss'">
            {{ signedMoney(autoStats.totalProfit) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="6">
        <el-card shadow="never" class="metric-card small">
          <div class="metric-label">自动化状态</div>
          <div class="metric-value small-value">{{ automation.enabled ? '已启用' : '已关闭' }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-tabs model-value="opportunities" class="main-tabs">
      <el-tab-pane label="机会扫描" name="opportunities">
        <el-table
          :data="opportunities"
          stripe
          v-loading="loading"
          class="data-table"
          empty-text="暂无可扫描组合：等待至少一条现货行情和一条永续行情同时在线"
        >
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="方向" min-width="170">
            <template #default="{ row }">
              <el-tag :type="row.direction === 'spot_long_perp_short' ? 'success' : 'warning'">
                {{ directionText(row.direction) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="组合" min-width="180">
            <template #default="{ row }">
              <span class="pair-text">{{ row.spotExchange }} 现货 / {{ row.perpExchange }} 永续</span>
            </template>
          </el-table-column>
          <el-table-column label="现货价" min-width="120">
            <template #default="{ row }">{{ money(row.spotPrice) }}</template>
          </el-table-column>
          <el-table-column label="永续价" min-width="120">
            <template #default="{ row }">{{ money(row.perpPrice) }}</template>
          </el-table-column>
          <el-table-column label="基差收益" min-width="110">
            <template #default="{ row }">
              <span :class="row.basisAmount >= 0 ? 'text-up' : 'text-down'">{{ signedMoney(row.basisAmount) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="资金费" min-width="100">
            <template #default="{ row }">
              <span :class="row.fundingAmount >= 0 ? 'text-up' : 'text-down'">{{ signedMoney(row.fundingAmount) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="持仓资金费" min-width="120">
            <template #default="{ row }">
              <span :class="row.carryFundingAmount >= 0 ? 'text-up' : 'text-down'">
                {{ signedMoney(row.carryFundingAmount) }}
              </span>
              <span class="muted"> / {{ row.carryFundingIntervals }}期</span>
            </template>
          </el-table-column>
          <el-table-column label="成本" min-width="130">
            <template #default="{ row }">{{ money(row.feeCost + row.slippage + row.safetyBuffer) }}</template>
          </el-table-column>
          <el-table-column label="净收益" min-width="120">
            <template #default="{ row }">
              <span :class="row.netProfit >= 0 ? 'text-up strong' : 'text-down strong'">{{ signedMoney(row.netProfit) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="收益率" min-width="90">
            <template #default="{ row }">{{ row.profitRate.toFixed(2) }}%</template>
          </el-table-column>
          <el-table-column label="持仓预期" min-width="130">
            <template #default="{ row }">
              <span :class="row.carryNetProfit >= 0 ? 'text-up strong' : 'text-down strong'">
                {{ signedMoney(row.carryNetProfit) }}
              </span>
              <span class="muted"> {{ row.carryProfitRate.toFixed(2) }}%</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" min-width="120">
            <template #default="{ row }">
              <el-tooltip v-if="row.status === 'blocked'" :content="row.blockReason" placement="top">
                <el-tag type="info">不可执行</el-tag>
              </el-tooltip>
              <el-tooltip v-else-if="row.status === 'watch'" :content="row.blockReason" placement="top">
                <el-tag type="warning">观察</el-tag>
              </el-tooltip>
              <el-tag v-else type="success">可执行</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button
                type="primary"
                size="small"
                :icon="CaretRight"
                :disabled="row.status !== 'ready' || status === 'halted'"
                @click="store.executeOpportunity(row)"
              >
                模拟开仓
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="机会记录" name="opportunity-logs">
        <el-table :data="opportunityLogs" stripe class="data-table" empty-text="暂无机会记录">
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="方向" min-width="170">
            <template #default="{ row }">{{ directionText(row.direction) }}</template>
          </el-table-column>
          <el-table-column label="组合" min-width="170">
            <template #default="{ row }">{{ row.spotExchange }} / {{ row.perpExchange }}</template>
          </el-table-column>
          <el-table-column label="出现次数" min-width="100">
            <template #default="{ row }">{{ row.seenCount }}</template>
          </el-table-column>
          <el-table-column label="最佳利润" min-width="130">
            <template #default="{ row }">{{ signedMoney(row.bestProfit) }} / {{ row.bestProfitRate.toFixed(2) }}%</template>
          </el-table-column>
          <el-table-column label="最近利润" min-width="130">
            <template #default="{ row }">{{ signedMoney(row.lastProfit) }} / {{ row.lastProfitRate.toFixed(2) }}%</template>
          </el-table-column>
          <el-table-column label="状态" min-width="100">
            <template #default="{ row }">
              <el-tag :type="row.lastStatus === 'ready' ? 'success' : row.lastStatus === 'watch' ? 'warning' : 'info'">
                {{ row.lastStatus === 'ready' ? '可执行' : row.lastStatus === 'watch' ? '观察' : '不可执行' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="自动开仓" min-width="100">
            <template #default="{ row }">{{ row.autoOpenedCount }}</template>
          </el-table-column>
          <el-table-column label="最近出现" min-width="180">
            <template #default="{ row }">{{ formatTs(row.lastSeenAt) }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="自动交易" name="auto-trades">
        <el-table :data="autoTrades" stripe class="data-table" empty-text="暂无自动交易记录">
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="动作" min-width="90">
            <template #default="{ row }">
              <el-tag :type="row.action === 'open' ? 'success' : 'warning'">{{ row.action === 'open' ? '开仓' : '平仓' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="方向" min-width="170">
            <template #default="{ row }">{{ directionText(row.direction) }}</template>
          </el-table-column>
          <el-table-column label="组合" min-width="170">
            <template #default="{ row }">{{ row.spotExchange }} / {{ row.perpExchange }}</template>
          </el-table-column>
          <el-table-column label="名义本金" min-width="120">
            <template #default="{ row }">{{ money(row.notional) }}</template>
          </el-table-column>
          <el-table-column label="保证金" min-width="120">
            <template #default="{ row }">{{ money(row.margin) }}</template>
          </el-table-column>
          <el-table-column label="资金占用" min-width="130">
            <template #default="{ row }">{{ money(row.capitalUsed) }}</template>
          </el-table-column>
          <el-table-column label="预计/实际利润" min-width="150">
            <template #default="{ row }">{{ signedMoney(row.profit) }} / {{ row.profitRate.toFixed(2) }}%</template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" min-width="260" />
          <el-table-column label="时间" min-width="180">
            <template #default="{ row }">{{ formatTs(row.createdAt) }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="行情源" name="quotes">
        <el-table :data="quotes" stripe class="data-table" empty-text="暂无行情快照">
          <el-table-column prop="exchange" label="交易所" min-width="110">
            <template #default="{ row }">
              <el-tag :type="exchangeType(row.exchange)">{{ row.exchange }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="市场" min-width="90">
            <template #default="{ row }">
              <el-tag :type="row.marketType === 'spot' ? 'success' : 'warning'">
                {{ row.marketType === 'spot' ? '现货' : '永续' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="买一价" min-width="120">
            <template #default="{ row }">{{ money(row.bid) }}</template>
          </el-table-column>
          <el-table-column label="卖一价" min-width="120">
            <template #default="{ row }">{{ money(row.ask) }}</template>
          </el-table-column>
          <el-table-column label="资金费率" min-width="110">
            <template #default="{ row }">
              {{ row.marketType === 'perp' ? `${(row.fundingRate * 100).toFixed(4)}%` : '-' }}
            </template>
          </el-table-column>
          <el-table-column label="状态" min-width="110">
            <template #default="{ row }">
              <el-tag :type="row.stale ? 'danger' : 'success'">{{ row.stale ? '过期' : '实时' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="年龄" min-width="110">
            <template #default="{ row }">{{ quoteAgeText(row.ageMillis) }}</template>
          </el-table-column>
          <el-table-column label="更新时间" min-width="180">
            <template #default="{ row }">{{ formatTs(row.timestamp) }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="虚拟资金" name="accounts">
        <div class="fund-manager">
          <el-form label-width="110px" class="fund-form">
            <el-form-item label="交易所">
              <el-select v-model="fundDraft.exchange" class="fund-select" @change="loadFundDraft">
                <el-option v-for="account in accounts" :key="account.exchange" :label="account.exchange" :value="account.exchange" />
              </el-select>
            </el-form-item>
            <el-form-item label="现货 USDT">
              <el-input-number v-model="fundDraft.usdt" :min="0" :precision="2" :step="100" />
            </el-form-item>
            <el-form-item label="合约 USDT">
              <el-input-number v-model="fundDraft.perpUsdt" :min="fundDraft.frozenUsdt" :precision="2" :step="100" />
              <span class="muted">需大于冻结保证金 {{ money(fundDraft.frozenUsdt) }}</span>
            </el-form-item>
            <el-form-item label="现货库存">
              <div class="inventory-editor">
                <div v-for="item in fundDraft.spotBalances" :key="item.asset" class="inventory-row">
                  <el-input v-model="item.asset" placeholder="BTC" />
                  <el-input-number v-model="item.amount" :min="0" :precision="6" :step="0.01" />
                  <el-button :icon="CloseBold" @click="removeInventoryAsset(item.asset)" />
                </div>
                <el-button @click="addInventoryAsset">添加库存币种</el-button>
              </div>
            </el-form-item>
            <el-form-item label="资金划转">
              <div class="transfer-row">
                <el-segmented v-model="transferDraft.direction" :options="transferOptions" />
                <el-input-number v-model="transferDraft.amount" :min="0" :precision="2" :step="100" />
                <el-button @click="handleTransfer">模拟划转</el-button>
              </div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveFundDraft">保存资金</el-button>
              <el-button @click="loadFundDraft">重载当前交易所</el-button>
              <el-button type="danger" plain @click="handleResetAccounts">恢复默认资金</el-button>
              <span class="muted">资金配置保存在后端模拟盘内存里，用于开仓前余额、库存和保证金检查。</span>
            </el-form-item>
          </el-form>
        </div>

        <el-table :data="accounts" stripe class="data-table">
          <el-table-column prop="exchange" label="交易所" min-width="110">
            <template #default="{ row }">
              <el-tag :type="exchangeType(row.exchange)">{{ row.exchange }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="现货 USDT" min-width="130">
            <template #default="{ row }">{{ money(row.usdt) }}</template>
          </el-table-column>
          <el-table-column label="合约 USDT" min-width="130">
            <template #default="{ row }">{{ money(row.perpUsdt) }}</template>
          </el-table-column>
          <el-table-column label="冻结保证金" min-width="130">
            <template #default="{ row }">{{ money(row.frozenUsdt) }}</template>
          </el-table-column>
          <el-table-column label="现货库存" min-width="260">
            <template #default="{ row }">
              <div class="asset-tags">
                <el-tag v-for="(amount, asset) in row.spotBalances" :key="asset" type="info">
                  {{ asset }} {{ amount.toFixed(4) }}
                </el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="永续持仓" min-width="260">
            <template #default="{ row }">
              <div class="asset-tags">
                <el-tag v-for="(amount, symbol) in row.perpPositions" :key="symbol" :type="amount >= 0 ? 'success' : 'danger'">
                  {{ symbol }} {{ amount.toFixed(4) }}
                </el-tag>
                <span v-if="Object.keys(row.perpPositions).length === 0" class="muted">无</span>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="模拟持仓" name="positions">
        <el-table :data="positions" stripe class="data-table">
          <el-table-column prop="symbol" label="交易对" min-width="110" />
          <el-table-column label="方向" min-width="170">
            <template #default="{ row }">{{ directionText(row.direction) }}</template>
          </el-table-column>
          <el-table-column label="组合" min-width="180">
            <template #default="{ row }">{{ row.spotExchange }} / {{ row.perpExchange }}</template>
          </el-table-column>
          <el-table-column label="数量" min-width="120">
            <template #default="{ row }">{{ row.quantity.toFixed(6) }}</template>
          </el-table-column>
          <el-table-column label="本金" min-width="120">
            <template #default="{ row }">{{ money(row.notional) }}</template>
          </el-table-column>
          <el-table-column label="保证金" min-width="120">
            <template #default="{ row }">{{ money(row.margin) }}</template>
          </el-table-column>
          <el-table-column label="预计净收益" min-width="130">
            <template #default="{ row }">
              <span class="text-up strong">{{ signedMoney(row.netProfit) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="已实现收益" min-width="130">
            <template #default="{ row }">
              <span :class="row.realizedPnL >= 0 ? 'text-up strong' : 'text-down strong'">
                {{ row.status === 'closed' ? signedMoney(row.realizedPnL) : '-' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="开仓时间" min-width="180">
            <template #default="{ row }">{{ formatTs(row.openedAt) }}</template>
          </el-table-column>
          <el-table-column prop="closedAt" label="平仓时间" min-width="180">
            <template #default="{ row }">
              <span class="muted">{{ row.closedAt ? formatTs(row.closedAt) : '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" min-width="100">
            <template #default="{ row }">
              <el-tag :type="positionStatusType(row.status)">
                {{ positionStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button
                size="small"
                type="warning"
                :icon="CloseBold"
                :disabled="row.status !== 'open'"
                @click="handleClosePosition(row.id)"
              >
                模拟平仓
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="熔断动作" name="close-actions">
        <el-table :data="closeActions" stripe class="data-table">
          <el-table-column prop="positionId" label="持仓 ID" min-width="180" />
          <el-table-column prop="reason" label="原因" min-width="180" />
          <el-table-column label="现货腿" min-width="140">
            <template #default="{ row }">{{ sideText(row.spotAction) }}</template>
          </el-table-column>
          <el-table-column label="永续腿" min-width="160">
            <template #default="{ row }">{{ sideText(row.perpAction) }}</template>
          </el-table-column>
          <el-table-column label="生成时间" min-width="180">
            <template #default="{ row }">{{ formatTs(row.createdAt) }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CaretRight, CloseBold, Refresh, Setting, SwitchButton, VideoPlay } from '@element-plus/icons-vue'
import { useCexSpotPerpStore, type SpotPerpAutomation, type SpotPerpConfig, type SpotPerpDirection } from '@/stores/cexSpotPerp'

const store = useCexSpotPerpStore()
const {
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
} = storeToRefs(store)

const allExchanges = ['binance', 'okx', 'bitget']
const commonSymbols = [
  'BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'SOLUSDT', 'XRPUSDT',
  'DOGEUSDT', 'ADAUSDT', 'TRXUSDT', 'LINKUSDT', 'AVAXUSDT',
  'TONUSDT', 'SHIBUSDT', 'DOTUSDT', 'BCHUSDT', 'LTCUSDT',
  'UNIUSDT', 'NEARUSDT', 'APTUSDT', 'ICPUSDT', 'ETCUSDT',
]
const configDraftStorageKey = 'cex-spot-perp-config-draft'
const automationDraftStorageKey = 'cex-spot-perp-automation-draft'

const cloneConfig = (value: SpotPerpConfig): SpotPerpConfig => ({
  ...value,
  symbols: [...value.symbols],
  exchanges: [...value.exchanges],
  exchangeSymbols: Object.fromEntries(
    Object.entries(value.exchangeSymbols || {}).map(([exchange, symbols]) => [exchange, [...symbols]])
  ),
})

const configKey = (value: SpotPerpConfig) => JSON.stringify({
  symbols: value.symbols,
  exchanges: value.exchanges,
  exchangeSymbols: value.exchangeSymbols,
  leverage: value.leverage,
})

const readStoredDraftConfig = (): SpotPerpConfig | null => {
  const raw = window.localStorage.getItem(configDraftStorageKey)
  if (!raw) return null
  try {
    return cloneConfig(JSON.parse(raw) as SpotPerpConfig)
  } catch {
    window.localStorage.removeItem(configDraftStorageKey)
    return null
  }
}

const automationKey = (value: SpotPerpAutomation) => JSON.stringify({
  enabled: value.enabled,
  autoOpen: value.autoOpen,
  autoClose: value.autoClose,
  openMinProfitRate: Number(value.openMinProfitRate || 0),
  closeMinProfitRate: Number(value.closeMinProfitRate || 0),
  maxHoldSeconds: Number(value.maxHoldSeconds || 0),
  maxOpenPositions: Number(value.maxOpenPositions || 0),
  checkIntervalMillis: Number(value.checkIntervalMillis || 0),
})

const normalizeAutomation = (value: SpotPerpAutomation): SpotPerpAutomation => ({
  enabled: Boolean(value.enabled),
  autoOpen: Boolean(value.autoOpen),
  autoClose: Boolean(value.autoClose),
  openMinProfitRate: Math.max(0, Number(value.openMinProfitRate || 0)),
  closeMinProfitRate: Math.max(0, Number(value.closeMinProfitRate || 0)),
  maxHoldSeconds: Math.max(0, Math.floor(Number(value.maxHoldSeconds || 0))),
  maxOpenPositions: Math.max(1, Math.floor(Number(value.maxOpenPositions || 1))),
  checkIntervalMillis: Math.max(500, Math.floor(Number(value.checkIntervalMillis || 1000))),
})

const readStoredAutomationDraft = (): SpotPerpAutomation | null => {
  const raw = window.localStorage.getItem(automationDraftStorageKey)
  if (!raw) return null
  try {
    return normalizeAutomation(JSON.parse(raw) as SpotPerpAutomation)
  } catch {
    window.localStorage.removeItem(automationDraftStorageKey)
    return null
  }
}

const storedDraftConfig = readStoredDraftConfig()
const draftConfig = reactive<SpotPerpConfig>(storedDraftConfig || cloneConfig(config.value))
const savedConfigKey = ref(configKey(config.value))
const isSyncingDraft = ref(false)
const hasUnsavedConfig = ref(Boolean(storedDraftConfig))
const storedAutomationDraft = readStoredAutomationDraft()
const automationDraft = reactive<SpotPerpAutomation>(storedAutomationDraft || normalizeAutomation(automation.value))
const savedAutomationKey = ref(automationKey(automation.value))
const isSyncingAutomationDraft = ref(false)
const hasUnsavedAutomation = ref(Boolean(storedAutomationDraft))
const fundDraft = reactive({
  exchange: 'binance',
  usdt: 0,
  perpUsdt: 0,
  frozenUsdt: 0,
  spotBalances: [] as Array<{ asset: string; amount: number }>,
})
const transferDraft = reactive({
  direction: 'spot_to_perp',
  amount: 0,
})
const transferOptions = [
  { label: '现货 -> 合约', value: 'spot_to_perp' },
  { label: '合约 -> 现货', value: 'perp_to_spot' },
]

const marketWSConnectedCount = computed(() => {
  return Object.values(wsStatus.value).filter(status => status === 'connected').length
})
const marketWSTotalCount = computed(() => Math.max(Object.keys(wsStatus.value).length, 5))

onMounted(() => {
  store.fetchSimulation()
  store.connectWebSocket()
})

watch(config, (value) => {
  savedConfigKey.value = configKey(value)
  if (hasUnsavedConfig.value) return
  isSyncingDraft.value = true
  Object.assign(draftConfig, cloneConfig(value))
  isSyncingDraft.value = false
}, { deep: true })

watch(draftConfig, () => {
  if (isSyncingDraft.value) return
  hasUnsavedConfig.value = configKey(draftConfig) !== savedConfigKey.value
  if (hasUnsavedConfig.value) {
    window.localStorage.setItem(configDraftStorageKey, JSON.stringify(draftConfig))
  } else {
    window.localStorage.removeItem(configDraftStorageKey)
  }
}, { deep: true })

watch(automation, (value) => {
  savedAutomationKey.value = automationKey(value)
  if (hasUnsavedAutomation.value) return
  isSyncingAutomationDraft.value = true
  Object.assign(automationDraft, normalizeAutomation(value))
  isSyncingAutomationDraft.value = false
}, { deep: true })

watch(automationDraft, () => {
  if (isSyncingAutomationDraft.value) return
  hasUnsavedAutomation.value = automationKey(automationDraft) !== savedAutomationKey.value
  if (hasUnsavedAutomation.value) {
    window.localStorage.setItem(automationDraftStorageKey, JSON.stringify(normalizeAutomation(automationDraft)))
  } else {
    window.localStorage.removeItem(automationDraftStorageKey)
  }
}, { deep: true })

watch(accounts, () => {
  if (!accounts.value.some(account => account.exchange === fundDraft.exchange)) {
    fundDraft.exchange = accounts.value[0]?.exchange || 'binance'
  }
  if (fundDraft.usdt === 0 && fundDraft.perpUsdt === 0 && fundDraft.spotBalances.length === 0) {
    loadFundDraft()
  }
}, { deep: true })

onUnmounted(() => {
  store.disconnectWebSocket()
})

const directionText = (direction: SpotPerpDirection) => {
  return direction === 'spot_long_perp_short' ? '买现货 + 空永续' : '卖库存 + 多永续'
}

const exchangeType = (exchange: string) => {
  const types: Record<string, string> = {
    binance: 'warning',
    okx: 'success',
    bitget: 'info',
  }
  return types[exchange] || ''
}

const money = (value: number) => {
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

const signedMoney = (value: number) => {
  return `${value >= 0 ? '+' : '-'}${money(Math.abs(value))}`
}

const normalizeDraftConfig = (): SpotPerpConfig => {
  const symbols = [...new Set(draftConfig.symbols.map(symbol => symbol.trim().toUpperCase()).filter(Boolean))]
  const exchanges = [...new Set(draftConfig.exchanges.filter(exchange => allExchanges.includes(exchange)))]
  const exchangeSymbols: Record<string, string[]> = {}
  for (const exchange of exchanges) {
    const configured = draftConfig.exchangeSymbols[exchange] || symbols
    exchangeSymbols[exchange] = configured.filter(symbol => symbols.includes(symbol))
    if (exchangeSymbols[exchange].length === 0) {
      exchangeSymbols[exchange] = [...symbols]
    }
  }
  return {
    ...draftConfig,
    symbols,
    exchanges,
    exchangeSymbols,
    leverage: Math.min(3, Math.max(1, Number(draftConfig.leverage || 1))),
  }
}

const applyTopSymbols = () => {
  draftConfig.symbols = [...commonSymbols]
  for (const exchange of draftConfig.exchanges) {
    draftConfig.exchangeSymbols[exchange] = [...commonSymbols]
  }
}

const saveConfig = async () => {
  const nextConfig = normalizeDraftConfig()
  if (nextConfig.symbols.length === 0 || nextConfig.exchanges.length === 0) {
    ElMessage.warning('至少配置一个交易所和一个币种')
    return
  }
  await store.updateConfig(nextConfig)
  isSyncingDraft.value = true
  Object.assign(draftConfig, cloneConfig(nextConfig))
  savedConfigKey.value = configKey(nextConfig)
  hasUnsavedConfig.value = false
  window.localStorage.removeItem(configDraftStorageKey)
  isSyncingDraft.value = false
  ElMessage.success('期现配置已保存')
}

const saveAutomation = async () => {
  const nextAutomation = normalizeAutomation(automationDraft)
  await store.updateAutomation(nextAutomation)
  isSyncingAutomationDraft.value = true
  Object.assign(automationDraft, nextAutomation)
  savedAutomationKey.value = automationKey(nextAutomation)
  hasUnsavedAutomation.value = false
  window.localStorage.removeItem(automationDraftStorageKey)
  isSyncingAutomationDraft.value = false
  ElMessage.success('自动化配置已保存')
}

const resetDraftConfig = () => {
  isSyncingDraft.value = true
  Object.assign(draftConfig, cloneConfig(config.value))
  window.localStorage.removeItem(configDraftStorageKey)
  hasUnsavedConfig.value = false
  isSyncingDraft.value = false
}

const resetAutomationDraft = () => {
  isSyncingAutomationDraft.value = true
  Object.assign(automationDraft, normalizeAutomation(automation.value))
  window.localStorage.removeItem(automationDraftStorageKey)
  hasUnsavedAutomation.value = false
  isSyncingAutomationDraft.value = false
}

const accountForFundDraft = () => {
  return accounts.value.find(account => account.exchange === fundDraft.exchange)
}

const loadFundDraft = () => {
  const account = accountForFundDraft()
  if (!account) return
  fundDraft.usdt = account.usdt
  fundDraft.perpUsdt = account.perpUsdt
  fundDraft.frozenUsdt = account.frozenUsdt
  fundDraft.spotBalances = Object.entries(account.spotBalances || {}).map(([asset, amount]) => ({
    asset,
    amount,
  }))
}

const addInventoryAsset = () => {
  fundDraft.spotBalances.push({ asset: '', amount: 0 })
}

const removeInventoryAsset = (asset: string) => {
  fundDraft.spotBalances = fundDraft.spotBalances.filter(item => item.asset !== asset)
}

const normalizedSpotBalances = () => {
  const balances: Record<string, number> = {}
  for (const item of fundDraft.spotBalances) {
    const asset = item.asset.trim().toUpperCase()
    if (!asset) continue
    balances[asset] = Math.max(0, Number(item.amount || 0))
  }
  return balances
}

const saveFundDraft = async () => {
  if (!fundDraft.exchange) return
  await store.updateAccount({
    exchange: fundDraft.exchange,
    usdt: Number(fundDraft.usdt || 0),
    perpUsdt: Number(fundDraft.perpUsdt || 0),
    frozenUsdt: fundDraft.frozenUsdt,
    spotBalances: normalizedSpotBalances(),
    perpPositions: accountForFundDraft()?.perpPositions || {},
  })
  loadFundDraft()
  ElMessage.success('模拟资金已保存')
}

const handleTransfer = async () => {
  if (!fundDraft.exchange || transferDraft.amount <= 0) {
    ElMessage.warning('请输入划转金额')
    return
  }
  const from = transferDraft.direction === 'spot_to_perp' ? 'spot' : 'perp'
  const to = transferDraft.direction === 'spot_to_perp' ? 'perp' : 'spot'
  await store.transferAccountUSDT(fundDraft.exchange, from, to, transferDraft.amount)
  transferDraft.amount = 0
  loadFundDraft()
  ElMessage.success('模拟划转完成')
}

const handleResetAccounts = async () => {
  await ElMessageBox.confirm('恢复默认资金会清空当前模拟持仓和熔断动作。', '确认恢复默认资金', {
    type: 'warning',
    confirmButtonText: '恢复默认',
    cancelButtonText: '取消',
  })
  await store.resetAccounts()
  loadFundDraft()
  ElMessage.success('模拟资金已恢复默认')
}

const quoteAgeText = (value: number) => {
  if (!value) return '-'
  if (value < 1000) return `${value}ms`
  return `${(value / 1000).toFixed(1)}s`
}

const handleHalt = async () => {
  await ElMessageBox.confirm('熔断会停止新开仓，并为所有模拟持仓生成紧急平仓动作。', '确认熔断', {
    type: 'warning',
    confirmButtonText: '熔断清仓',
    cancelButtonText: '取消',
  })
  await store.triggerCircuitBreaker('手动熔断')
}

const positionStatusText = (status: string) => {
  const text: Record<string, string> = {
    open: '持仓中',
    closing: '平仓中',
    closed: '已平仓',
  }
  return text[status] || status
}

const positionStatusType = (status: string) => {
  const type: Record<string, string> = {
    open: 'success',
    closing: 'warning',
    closed: 'info',
  }
  return type[status] || 'info'
}

const handleClosePosition = async (positionId: string) => {
  await ElMessageBox.confirm('模拟平仓会反向处理现货腿和永续腿，并释放冻结保证金。', '确认平仓', {
    type: 'warning',
    confirmButtonText: '模拟平仓',
    cancelButtonText: '取消',
  })
  await store.closePosition(positionId)
}

const sideText = (side: string) => {
  const text: Record<string, string> = {
    buy: '买入',
    sell: '卖出',
  }
  return text[side] || side
}

const formatTs = (value?: number) => {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}
</script>

<style scoped>
.spot-perp-page {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

h2 {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 650;
  color: #1f2937;
}

.subtitle,
.muted {
  color: #6b7280;
  font-size: 13px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.halt-alert,
.summary-row {
  margin-bottom: 16px;
}

.config-panel {
  margin-bottom: 16px;
  border-radius: 6px;
}

.config-form {
  max-width: 980px;
}

.leverage-control {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) 140px;
  gap: 16px;
  width: 100%;
  max-width: 520px;
  align-items: center;
}

.wide-select {
  width: 100%;
  max-width: 520px;
}

.symbol-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  width: 100%;
  align-items: center;
}

.exchange-symbols {
  display: grid;
  gap: 10px;
  width: 100%;
  max-width: 680px;
}

.exchange-symbol-row {
  display: grid;
  grid-template-columns: 82px minmax(0, 1fr);
  gap: 12px;
  align-items: center;
}

.exchange-label {
  color: #374151;
  font-weight: 650;
  text-transform: capitalize;
}

.exchange-symbol-select {
  width: 100%;
}

.fund-manager {
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  background: #fff;
}

.fund-form {
  max-width: 980px;
}

.fund-select {
  width: 220px;
}

.inventory-editor {
  display: grid;
  gap: 10px;
  width: 100%;
  max-width: 560px;
}

.inventory-row {
  display: grid;
  grid-template-columns: 140px 220px 40px;
  gap: 10px;
  align-items: center;
}

.transfer-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.automation-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.summary-row.secondary {
  margin-top: -8px;
}

.metric-card {
  min-height: 92px;
  margin-bottom: 12px;
}

.metric-card.small {
  min-height: 72px;
}

.metric-label {
  color: #6b7280;
  font-size: 13px;
  margin-bottom: 8px;
}

.metric-value {
  color: #111827;
  font-size: 24px;
  font-weight: 700;
}

.metric-value.compact {
  font-size: 18px;
}

.metric-value.profit {
  color: #16a34a;
}

.metric-value.loss {
  color: #dc2626;
}

.small-value {
  font-size: 18px;
}

.main-tabs {
  background: #fff;
  padding: 0 16px 16px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.data-table {
  width: 100%;
}

.pair-text {
  color: #374151;
  font-weight: 500;
}

.asset-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.text-up {
  color: #16a34a;
}

.text-down {
  color: #dc2626;
}

.strong {
  font-weight: 650;
}

@media (max-width: 900px) {
  .page-header {
    display: block;
  }

  .header-actions {
    justify-content: flex-start;
    margin-top: 12px;
  }

  .leverage-control,
  .exchange-symbol-row,
  .inventory-row {
    grid-template-columns: 1fr;
  }
}
</style>
