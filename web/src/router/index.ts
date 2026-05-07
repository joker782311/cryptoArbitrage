import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: () => import('../views/Dashboard.vue'),
    },
    {
      path: '/strategies',
      name: 'strategies',
      component: () => import('../views/Strategies.vue'),
    },
    {
      path: '/cex-spot-perp',
      name: 'cex-spot-perp',
      component: () => import('../views/CexSpotPerp.vue'),
    },
    {
      path: '/positions',
      name: 'positions',
      component: () => import('../views/Positions.vue'),
    },
    {
      path: '/alerts',
      name: 'alerts',
      component: () => import('../views/Alerts.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/Settings.vue'),
    },
  ],
})

export default router
