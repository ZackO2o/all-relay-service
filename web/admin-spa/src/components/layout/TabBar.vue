<template>
  <div class="mb-4">
    <!-- Mobile: select dropdown -->
    <div class="sm:hidden">
      <select
        class="w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
        :value="activeTab"
        @change="$emit('tab-change', $event.target.value)"
      >
        <option v-for="tab in tabs" :key="tab.key" :value="tab.key">{{ tab.name }}</option>
      </select>
    </div>

    <!-- Desktop: tab buttons -->
    <div class="hidden flex-wrap gap-1 sm:flex">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        class="tab-btn"
        :class="activeTab === tab.key ? 'active' : ''"
        @click="$emit('tab-change', tab.key)"
      >
        <i :class="tab.icon +  mr-1.5" />
        {{ tab.name }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue"
import { useAuthStore } from "@/stores/auth"

defineProps({ activeTab: { type: String, required: true } })
defineEmits(["tab-change"])

const authStore = useAuthStore()
const tabs = computed(() => {
  const t = [
    { key: "dashboard", name: "仪表板", icon: "fas fa-tachometer-alt" },
    { key: "apiKeys", name: "API Keys", icon: "fas fa-key" },
    { key: "accounts", name: "账户管理", icon: "fas fa-user-circle" },
    { key: "requestDetails", name: "请求明细", icon: "fas fa-table" },
    { key: "quotaCards", name: "额度卡", icon: "fas fa-ticket-alt" },
  ]
  if (authStore.oemSettings?.ldapEnabled) {
    t.push({ key: "userManagement", name: "用户管理", icon: "fas fa-users" })
  }
  t.push({ key: "settings", name: "系统设置", icon: "fas fa-cogs" })
  return t
})
</script>
