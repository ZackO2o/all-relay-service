<template>
  <div class="mx-auto min-h-screen max-w-7xl px-3 py-4 sm:px-6 sm:py-6">
    <AppHeader />

    <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800 sm:p-6">
      <TabBar :active-tab="activeTab" @tab-change="handleTabChange" />
      <div class="tab-content">
        <router-view />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed } from "vue"
import { useRoute, useRouter } from "vue-router"
import { useAuthStore } from "@/stores/auth"
import AppHeader from "./AppHeader.vue"
import TabBar from "./TabBar.vue"

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const activeTab = ref("dashboard")

const tabRouteMap = computed(() => {
  const m = { dashboard: "/dashboard", apiKeys: "/api-keys", accounts: "/accounts", requestDetails: "/request-details", quotaCards: "/quota-cards", settings: "/settings" }
  if (authStore.oemSettings?.ldapEnabled) m.userManagement = "/user-management"
  return m
})

const initActiveTab = () => {
  const k = Object.keys(tabRouteMap.value).find((k) => tabRouteMap.value[k] === route.path)
  activeTab.value = k || "dashboard"
}

const handleTabChange = async (key) => {
  if (tabRouteMap.value[key] === route.path) return
  activeTab.value = key
  try { await router.push(tabRouteMap.value[key]); await nextTick() }
  catch (err) { if (err.name !== "NavigationDuplicated") initActiveTab() }
}

initActiveTab()
watch(() => route.path, () => initActiveTab())
</script>
