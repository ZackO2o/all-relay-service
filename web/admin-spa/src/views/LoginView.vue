<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-gray-900">
    <div class="fixed right-4 top-4 z-10">
      <ThemeToggle mode="dropdown" />
    </div>

    <div class="w-full max-w-sm">
      <div class="mb-6 text-center">
        <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-lg bg-blue-100 dark:bg-blue-900/40">
          <img
            v-if="!oemLoading && (authStore.oemSettings.siteIconData || authStore.oemSettings.siteIcon)"
            alt="Logo"
            class="h-6 w-6 object-contain"
            :src="authStore.oemSettings.siteIconData || authStore.oemSettings.siteIcon"
          />
          <svg v-else class="h-6 w-6 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path d="M13 10V3L4 14h7v7l9-11h-7z" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" />
          </svg>
        </div>
        <h1 v-if="!oemLoading && authStore.oemSettings.siteName" class="text-xl font-bold text-gray-900 dark:text-white">
          {{ authStore.oemSettings.siteName }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">管理后台</p>
      </div>

      <div class="rounded-lg bg-white px-6 py-8 shadow dark:bg-gray-800">
        <form class="space-y-5" @submit.prevent="handleLogin">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="username">用户名</label>
            <input
              id="username"
              v-model="loginForm.username"
              type="text"
              autocomplete="username"
              class="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:placeholder-gray-500 dark:focus:border-blue-400"
              placeholder="请输入用户名"
              required
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="password">密码</label>
            <input
              id="password"
              v-model="loginForm.password"
              type="password"
              autocomplete="current-password"
              class="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:placeholder-gray-500 dark:focus:border-blue-400"
              placeholder="请输入密码"
              required
            />
          </div>

          <div v-if="authStore.loginError" class="rounded-md bg-red-50 p-3 dark:bg-red-900/20">
            <p class="text-sm text-red-700 dark:text-red-400">
              <i class="fas fa-exclamation-triangle mr-1" />{{ authStore.loginError }}
            </p>
          </div>

          <button
            type="submit"
            class="btn btn-primary w-full py-2.5 text-sm"
            :disabled="authStore.loginLoading"
          >
            <div v-if="authStore.loginLoading" class="loading-spinner mr-2" />
            <i v-else class="fas fa-sign-in-alt mr-2" />
            {{ authStore.loginLoading ? '登录中...' : '登录' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from "vue"
import { useAuthStore } from "@/stores/auth"
import { useThemeStore } from "@/stores/theme"
import ThemeToggle from "@/components/common/ThemeToggle.vue"

const authStore = useAuthStore()
const themeStore = useThemeStore()
const oemLoading = computed(() => authStore.oemLoading)

const loginForm = ref({ username: "", password: "" })

onMounted(() => {
  themeStore.initTheme()
  authStore.loadOemSettings()
})

const handleLogin = async () => {
  await authStore.login(loginForm.value)
}
</script>
