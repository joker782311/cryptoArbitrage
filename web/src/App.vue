<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, RouterView } from 'vue-router'

const activeMenu = ref('dashboard')

const menuItems = [
  { key: 'dashboard', name: '行情看板', icon: '📊', path: '/' },
  { key: 'strategies', name: '策略配置', icon: '⚙️', path: '/strategies' },
  { key: 'positions', name: '仓位管理', icon: '💼', path: '/positions' },
  { key: 'alerts', name: '告警中心', icon: '🔔', path: '/alerts' },
  { key: 'settings', name: '系统设置', icon: '🔧', path: '/settings' },
]
</script>

<template>
  <div class="app">
    <aside class="sidebar">
      <div class="logo">
        <h2>🚀 CryptoArbitrage</h2>
      </div>
      <nav class="menu">
        <RouterLink
          v-for="item in menuItems"
          :key="item.key"
          :to="item.path"
          class="menu-item"
          active-class="active"
        >
          <span class="icon">{{ item.icon }}</span>
          <span class="name">{{ item.name }}</span>
        </RouterLink>
      </nav>
    </aside>

    <main class="main-content">
      <RouterView />
    </main>
  </div>
</template>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body {
  width: 100%;
  height: 100%;
  overflow-x: hidden;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
  background: #f5f7fa;
}

.app {
  display: flex;
  width: 100%;
  height: 100vh;
  overflow: hidden;
}

.sidebar {
  width: 220px;
  background: #001529;
  padding: 20px 0;
  position: fixed;
  height: 100vh;
  overflow-y: auto;
  z-index: 1000;
}

.logo {
  padding: 0 20px 20px;
  border-bottom: 1px solid #002140;
  margin-bottom: 20px;
}

.logo h2 {
  color: #fff;
  font-size: 18px;
  font-weight: 600;
}

.menu {
  display: flex;
  flex-direction: column;
}

.menu-item {
  display: flex;
  align-items: center;
  padding: 12px 20px;
  color: rgba(255, 255, 255, 0.65);
  text-decoration: none;
  transition: all 0.3s;
}

.menu-item:hover {
  color: #fff;
  background: #002140;
}

.menu-item.active {
  color: #fff;
  background: #1890ff;
}

.icon {
  font-size: 18px;
  margin-right: 10px;
}

.main-content {
  margin-left: 220px;
  width: calc(100vw - 220px);
  height: 100vh;
  overflow-y: auto;
  background: #f5f7fa;
  padding: 0;
}
</style>
