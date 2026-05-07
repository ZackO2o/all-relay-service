<template>
  <div class="mb-4 rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-gray-700 dark:bg-gray-800">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="flex h-8 w-8 items-center justify-center rounded-md bg-blue-100 dark:bg-blue-900/40">
          <svg class="h-4 w-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path d="M13 10V3L4 14h7v7l9-11h-7z" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" />
          </svg>
        </div>
        <div>
          <div class="flex items-center gap-2">
            <span class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ oemSettings.siteName || "ALL Relay" }}
            </span>
            <span class="font-mono text-xs text-gray-400">v{{ versionInfo.current || "..." }}</span>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">管理后台</p>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <ThemeToggle mode="dropdown" />
        <div class="h-5 w-px bg-gray-200 dark:bg-gray-700" />
        <div class="relative">
          <button
            class="flex items-center gap-1.5 rounded-md bg-gray-100 px-3 py-1.5 text-sm text-gray-700 transition-colors hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
            @click="userMenuOpen = !userMenuOpen"
          >
            <i class="fas fa-user-circle text-sm" />
            <span class="hidden sm:inline text-sm">{{ currentUser.username || 'Admin' }}</span>
            <i class="fas fa-chevron-down text-xs" :class="{ 'rotate-180': userMenuOpen }" />
          </button>

          <div
            v-if="userMenuOpen"
            class="absolute right-0 top-full z-50 mt-1 w-48 rounded-md border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
            @click.stop
          >
            <div class="border-b border-gray-100 px-3 py-2 dark:border-gray-700">
              <p class="text-xs text-gray-500 dark:text-gray-400">版本 v{{ versionInfo.current || "..." }}</p>
            </div>
            <button
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700"
              @click="openChangePasswordModal"
            >
              <i class="fas fa-key w-4 text-blue-500" />
              <span>修改账户</span>
            </button>
            <hr class="my-1 border-gray-100 dark:border-gray-700" />
            <button
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700"
              @click="handleLogoutClick"
            >
              <i class="fas fa-sign-out-alt w-4 text-red-500" />
              <span>退出登录</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Change Password Modal -->
  <div v-if="showChangePasswordModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
    <div class="w-full max-w-md rounded-lg bg-white p-6 shadow-lg dark:bg-gray-800">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-lg font-bold text-gray-900 dark:text-white">修改账户信息</h3>
        <button class="text-gray-400 hover:text-gray-600" @click="closeChangePasswordModal">
          <i class="fas fa-times" />
        </button>
      </div>
      <form class="space-y-4" @submit.prevent="changePassword">
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">当前用户名</label>
          <input class="w-full rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm text-gray-500 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-400" type="text" :value="currentUser.username || 'Admin'" disabled />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">新用户名</label>
          <input v-model="changePasswordForm.newUsername" class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" placeholder="留空保持不变" type="text" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">当前密码</label>
          <input v-model="changePasswordForm.currentPassword" class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" type="password" placeholder="请输入当前密码" required />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">新密码</label>
          <input v-model="changePasswordForm.newPassword" class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" type="password" placeholder="至少8位" required />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">确认新密码</label>
          <input v-model="changePasswordForm.confirmPassword" class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" type="password" placeholder="再次输入" required />
        </div>
        <div class="flex gap-3 pt-2">
          <button type="button" class="flex-1 rounded-md border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600" @click="closeChangePasswordModal">取消</button>
          <button type="submit" class="btn btn-primary flex-1 py-2 text-sm" :disabled="changePasswordLoading">
            <div v-if="changePasswordLoading" class="loading-spinner mr-2" />
            <i v-else class="fas fa-save mr-2" />
            {{ changePasswordLoading ? "保存中..." : "保存修改" }}
          </button>
        </div>
      </form>
    </div>
  </div>

  <ConfirmModal
    :title="confirmModalConfig.title"
    :message="confirmModalConfig.message"
    :confirm-text="confirmModalConfig.confirmText"
    :cancel-text="confirmModalConfig.cancelText"
    :type="confirmModalConfig.type"
    :show="showConfirmModal"
    @confirm="handleConfirmModal"
    @cancel="handleCancelModal"
  />
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from "vue"
import { useRouter } from "vue-router"
import { useAuthStore } from "@/stores/auth"
import { showToast } from "@/utils/tools"
import { checkUpdatesApi, changePasswordApi } from "@/utils/http_apis"
import ThemeToggle from "@/components/common/ThemeToggle.vue"
import ConfirmModal from "@/components/common/ConfirmModal.vue"

const router = useRouter()
const authStore = useAuthStore()
const currentUser = computed(() => authStore.user || { username: "Admin" })
const oemSettings = computed(() => authStore.oemSettings || {})

const versionInfo = ref({
  current: "...",
  latest: "",
  hasUpdate: false,
  checkingUpdate: false,
  releaseInfo: null,
  noUpdateMessage: false,
})

const userMenuOpen = ref(false)
const showChangePasswordModal = ref(false)
const changePasswordLoading = ref(false)
const showConfirmModal = ref(false)
const confirmModalConfig = ref({ title: "", message: "", type: "primary", confirmText: "确认", cancelText: "取消" })
const confirmResolve = ref(null)
const changePasswordForm = reactive({ currentPassword: "", newPassword: "", confirmPassword: "", newUsername: "" })

const showConfirm = (title, message, confirmText, cancelText, type) => {
  return new Promise((resolve) => {
    confirmModalConfig.value = { title, message, confirmText, cancelText, type }
    confirmResolve.value = resolve
    showConfirmModal.value = true
  })
}
const handleConfirmModal = () => { showConfirmModal.value = false; confirmResolve.value?.(true) }
const handleCancelModal = () => { showConfirmModal.value = false; confirmResolve.value?.(false) }

const handleLogoutClick = async () => {
  const ok = await showConfirm("退出登录", "确定要退出登录吗？", "确定退出", "取消", "warning")
  if (ok) { authStore.logout(); router.push("/login"); showToast("已安全退出", "success") }
  userMenuOpen.value = false
}

const checkForUpdates = async () => {
  if (versionInfo.value.checkingUpdate) return
  versionInfo.value.checkingUpdate = true
  try {
    const r = await checkUpdatesApi()
    if (r?.success) {
      Object.assign(versionInfo.value, { current: r.data.current, latest: r.data.latest, hasUpdate: r.data.hasUpdate, releaseInfo: r.data.releaseInfo })
      if (!r.data.hasUpdate) { versionInfo.value.noUpdateMessage = true; setTimeout(() => { versionInfo.value.noUpdateMessage = false }, 3000) }
    }
  } catch (e) {
    const cached = localStorage.getItem("versionInfo")
    if (cached) { try { Object.assign(versionInfo.value, JSON.parse(cached)) } catch {} }
  } finally { versionInfo.value.checkingUpdate = false }
}

const openChangePasswordModal = () => {
  Object.assign(changePasswordForm, { currentPassword: "", newPassword: "", confirmPassword: "", newUsername: "" })
  showChangePasswordModal.value = true; userMenuOpen.value = false
}
const closeChangePasswordModal = () => { showChangePasswordModal.value = false }
const changePassword = async () => {
  if (changePasswordForm.newPassword !== changePasswordForm.confirmPassword) { showToast("两次输入的密码不一致", "error"); return }
  if (changePasswordForm.newPassword.length < 8) { showToast("新密码长度至少8位", "error"); return }
  changePasswordLoading.value = true
  try {
    const d = await changePasswordApi({ currentPassword: changePasswordForm.currentPassword, newPassword: changePasswordForm.newPassword, newUsername: changePasswordForm.newUsername || undefined })
    if (d?.success) { showToast("修改成功，请重新登录", "success"); closeChangePasswordModal(); setTimeout(() => { authStore.logout(); router.push("/login") }, 1500) }
    else { showToast(d?.message || "修改失败", "error") }
  } catch { showToast("修改失败", "error") } finally { changePasswordLoading.value = false }
}

const handleClickOutside = (e) => { if (!e.target.closest(".user-menu-container") && userMenuOpen.value) userMenuOpen.value = false }

onMounted(() => { checkForUpdates(); document.addEventListener("click", handleClickOutside) })
onUnmounted(() => { document.removeEventListener("click", handleClickOutside) })
</script>
