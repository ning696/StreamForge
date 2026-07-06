<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { register } from '@/api/user'
import { getErrorMessage } from '@/api/http'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({
  account: '',
  password: '',
  username: '',
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
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 1, max: 32, message: '用户名长度必须为 1-32', trigger: 'blur' },
  ],
}

async function submit() {
  await formRef.value?.validate()
  loading.value = true
  try {
    const user = await register(form)
    authStore.setUser(user)
    await router.push('/')
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
        <h1>创建账号</h1>
        <p>MVP 只保存账号、密码哈希和用户名，不引入 JWT、Session 或 OAuth2。</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号" prop="account">
          <el-input v-model.trim="form.account" autocomplete="username" />
        </el-form-item>
        <el-form-item label="用户名" prop="username">
          <el-input v-model.trim="form.username" autocomplete="name" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" autocomplete="new-password" show-password />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading">注册并进入</el-button>
      </el-form>
      <p class="switch">已有账号？<RouterLink to="/login">登录</RouterLink></p>
    </section>
  </main>
</template>
