<template>
  <v-container
      class="d-flex justify-center align-center"
      style="height: 100vh"
  >
    <v-row justify="center" align="center" class="w-100">
      <v-col cols="12" md="6" lg="4">
        <v-card class="pa-6 rounded-xl" elevation="8">
<!--          <div class="text-center mb-6">-->
<!--            <img  src="~/assets/images/425logo.jpeg" alt="Logo" height="60" style="border-radius: 14px" />-->
<!--          </div>-->

          <!-- Title -->
          <div class="text-start">
            <h4 class="text-h6 font-weight-bold mb-1">Welcome Back!</h4>
            <p class="text-subtitle-2 text-grey">Sign in to continue to WMS.</p>
          </div>

          <!-- Form -->
          <v-form @submit.prevent="handleLogin" ref="loginForm">
            <v-text-field
                v-model="email"
                label="Username"
                prepend-inner-icon="mdi-account"
                variant="outlined"
                density="comfortable"
                class="mt-4"
                ref="emailInput"
                :rules="[rules.required]"
                @keydown.enter="handleLogin"
            />

            <v-text-field
                v-model="password"
                label="Password"
                prepend-inner-icon="mdi-lock"
                type="password"
                variant="outlined"
                density="comfortable"
                class="mt-2"
                ref="passwordInput"
                :rules="[rules.required]"
                @keydown.enter="handleLogin"
            />

            <ButtonBase
                :loading="loading"
                label="SIGN IN"
                @click="handleLogin"
                type="submit"
            />

            <v-divider class="my-6" />

            <!-- Signup -->
            <div class="mt-4 text-center">
              <p class="text-caption">
                Don't have an account?
              </p>
              <v-btn variant="outlined" color="primary" rounded>Signup now</v-btn>
            </div>
          </v-form>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Swal from 'sweetalert2'
import { useAuthStore } from "~/stores/auth.js";
import ButtonBase from "~/components/ButtonBase.vue";

const email = ref('')
const password = ref('')
const emailInput = ref(null)
const passwordInput = ref(null)
const router = useRouter()
const authStore = useAuthStore()
const loading = ref(false)
const rules = {
  required: (v) => !!v || 'This field is required.',
}

const handleLogin = async () => {
  if (!email.value || !password.value) {

    await nextTick()
    if (!email.value && emailInput.value) {
      emailInput.value.focus()
    } else if (!password.value && passwordInput.value) {
      passwordInput.value.focus()
    }

    return
  }

  loading.value = true
  try {
    const tokenRes = await $fetch('http://127.0.0.1:8000/api/token/', {
      method: 'POST',
      body: {
        username: email.value,
        password: password.value
      }
    })

    const accessToken = tokenRes.access

    const userRes = await $fetch('http://127.0.0.1:8000/api/user', {
      headers: {
        Authorization: `JWT ${accessToken}`
      }
    })

    localStorage.setItem('access_token', accessToken)
    localStorage.setItem('refresh_token', tokenRes.refresh)
    localStorage.setItem('User', JSON.stringify(userRes.User))
    localStorage.setItem('email', userRes.User.email)
    localStorage.setItem('username', userRes.User.username)
    localStorage.setItem('default_branch', userRes.User.default_branch)
    localStorage.setItem('is_staff', userRes.User.is_staff)

    authStore.setUser(userRes.User)
    authStore.setToken(accessToken)
    await router.push('/')

  } catch (err) {
    if (err?.status === 401) {
      Swal.fire("Invalid email or password. Please try again.")
    } else {
      Swal.fire("Login failed", err?.data?.detail || err.message, 'error')
    }
  } finally {
    loading.value = false
  }
}

</script>


<style scoped>
.text-grey {
  color: #6c757d;
}
</style>
