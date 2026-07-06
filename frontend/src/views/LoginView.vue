<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { login } from '@/api/user'
import { getErrorMessage } from '@/api/http'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({
  account: '',
  password: '',
})

const rules: FormRules = {
  account: [
    { required: true, message: '请输入账号', trigger: 'blur' },
    { min: 4, max: 32, message: '账号长度必须为 4-32', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 64, message: '密码长度必须为 6-64', trigger: 'blur' },
  ],
}

async function submit() {
  await formRef.value?.validate()
  loading.value = true
  try {
    const user = await login(form)
    authStore.setUser(user)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(redirect)
  } catch (error) {
    ElMessage.error(getErrorMessage(error))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div>
        <h1>StreamForge</h1>
        <p>登录后创建房间、加入会议，并验证 Week 1-2 的信令与聊天链路。</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号" prop="account">
          <el-input v-model.trim="form.account" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" autocomplete="current-password" show-password />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading">登录</el-button>
      </el-form>
      <p class="switch">还没有账号？<RouterLink to="/register">注册</RouterLink></p>
    </section>
  </main>
</template>
