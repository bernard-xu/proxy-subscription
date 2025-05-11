<template>
  <div class="app-header">
    <div class="logo">
      <span class="logo-text">代理订阅管理系统</span>
    </div>
    
    <div class="header-actions">
      <el-tooltip content="刷新" placement="bottom">
        <el-button circle @click="refreshPage" :icon="Refresh" size="small"></el-button>
      </el-tooltip>
      <el-tooltip content="全屏" placement="bottom">
        <el-button circle @click="toggleFullScreen" :icon="FullScreen" size="small"></el-button>
      </el-tooltip>
      
      <template v-if="authStore.isAuthenticated">
        <el-dropdown @command="handleCommand" trigger="click">
          <div class="user-info">
            <el-avatar :size="32" class="user-avatar">
              {{ authStore.user?.username?.charAt(0).toUpperCase() }}
            </el-avatar>
            <span class="username">{{ authStore.user?.username }}</span>
            <el-icon><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="changePassword">
                <el-icon><Key /></el-icon>
                <span>修改密码</span>
              </el-dropdown-item>
              <el-dropdown-item command="logout" divided>
                <el-icon><SwitchButton /></el-icon>
                <span>退出登录</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>
    </div>
    
    <!-- 修改密码对话框 -->
    <el-dialog v-model="passwordDialogVisible" title="修改密码" width="400px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="原密码" prop="oldPassword">
          <el-input v-model="form.oldPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="form.newPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认新密码" prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="loading" @click="changePassword">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { Refresh, FullScreen, ArrowDown, Key, SwitchButton } from '@element-plus/icons-vue';

const router = useRouter();
const authStore = useAuthStore();
const passwordDialogVisible = ref(false);
const formRef = ref<FormInstance>();
const loading = ref(false);

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
});

// 表单验证规则
const rules: FormRules = {
  oldPassword: [
    { required: true, message: '请输入原密码', trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少6个字符', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== form.newPassword) {
          callback(new Error('两次输入的密码不一致'));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

// 处理下拉菜单命令
const handleCommand = (command: string) => {
  if (command === 'logout') {
    logout();
  } else if (command === 'changePassword') {
    passwordDialogVisible.value = true;
  }
};

// 退出登录
const logout = () => {
  authStore.logout();
  router.push('/login');
  ElMessage.success('已成功退出登录');
};

// 修改密码
const changePassword = async () => {
  if (!formRef.value) return;
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true;
      try {
        await authStore.changePassword(form.oldPassword, form.newPassword);
        ElMessage.success('密码修改成功');
        passwordDialogVisible.value = false;
        // 清空表单
        form.oldPassword = '';
        form.newPassword = '';
        form.confirmPassword = '';
      } catch (error: any) {
        ElMessage.error(error.message || '修改密码失败');
      } finally {
        loading.value = false;
      }
    }
  });
};

// 刷新页面
const refreshPage = () => {
  window.location.reload();
};

// 切换全屏
const toggleFullScreen = () => {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen();
  } else {
    if (document.exitFullscreen) {
      document.exitFullscreen();
    }
  }
};
</script>

<style scoped>
.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 16px;
  height: 60px;
  width: 100%;
  background-color: #fff;
}

.logo {
  display: flex;
  align-items: center;
}

.logo-img {
  height: 36px;
  margin-right: 10px;
}

.logo-text {
  font-size: 18px;
  font-weight: bold;
  color: #409EFF;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  padding: 2px 8px;
  border-radius: 4px;
  transition: background-color 0.3s;
}

.user-info:hover {
  background-color: #f5f7fa;
}

.user-avatar {
  background-color: #409EFF;
  color: white;
}

.username {
  margin: 0 8px;
  color: #606266;
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style> 