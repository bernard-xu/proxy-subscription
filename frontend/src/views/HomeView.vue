<template>
  <div class="home-container">
    <el-card class="welcome-card">
      <template #header>
        <div class="card-header">
          <h2>欢迎使用 NekoRay 配置管理</h2>
        </div>
      </template>
      <div class="card-content">
        <p>这是一个用于管理代理订阅的工具，您可以：</p>
        <ul>
          <li>添加和管理多个订阅源</li>
          <li>查看所有代理节点</li>
          <li>获取合并后的订阅链接</li>
        </ul>
        
        <el-divider />
        
        <div class="merged-subscription">
          <h3>合并订阅链接</h3>
          <p>您可以使用以下链接在 NekoBox 中导入所有启用的订阅：</p>
          
          <div class="subscription-links">
            <div class="link-item">
              <span class="link-label">Base64 格式：</span>
              <el-input
                v-model="base64Url"
                readonly
                class="link-input"
              >
                <template #append>
                  <el-button @click="copyToClipboard(base64Url)">
                    复制
                  </el-button>
                </template>
              </el-input>
            </div>
            
            <div class="link-item">
              <span class="link-label">Clash 格式：</span>
              <el-input
                v-model="clashUrl"
                readonly
                class="link-input"
              >
                <template #append>
                  <el-button @click="copyToClipboard(clashUrl)">
                    复制
                  </el-button>
                </template>
              </el-input>
            </div>
          </div>
        </div>
        
        <el-divider />
        
        <div class="quick-stats">
          <h3>快速统计</h3>
          <el-row :gutter="20">
            <el-col :span="8">
              <el-statistic title="订阅数量" :value="subscriptionCount" />
            </el-col>
            <el-col :span="8">
              <el-statistic title="启用订阅" :value="enabledSubscriptionCount" />
            </el-col>
            <el-col :span="8">
              <el-statistic title="节点总数" :value="proxyCount" />
            </el-col>
          </el-row>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { useSubscriptionStore } from '@/stores/subscription';
import { useProxyStore } from '@/stores/proxy';
import { getMergedSubscriptionUrl } from '@/api';

const subscriptionStore = useSubscriptionStore();
const proxyStore = useProxyStore();

// 订阅统计
const subscriptionCount = computed(() => subscriptionStore.subscriptions.length);
const enabledSubscriptionCount = computed(() => subscriptionStore.enabledSubscriptions.length);
const proxyCount = computed(() => proxyStore.proxies.length);

// 合并订阅链接
const base64Url = ref(getMergedSubscriptionUrl('base64'));
const clashUrl = ref(getMergedSubscriptionUrl('clash'));

// 复制到剪贴板
const copyToClipboard = (text: string) => {
  navigator.clipboard.writeText(text)
    .then(() => {
      ElMessage.success('已复制到剪贴板');
    })
    .catch(() => {
      ElMessage.error('复制失败');
    });
};

// 加载数据
onMounted(async () => {
  await subscriptionStore.fetchSubscriptions();
  await proxyStore.fetchProxies();
});
</script>

<style scoped>
.home-container {
  max-width: 1000px;
  margin: 0 auto;
}

.welcome-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-content {
  padding: 10px 0;
}

.merged-subscription {
  margin: 20px 0;
}

.subscription-links {
  margin-top: 15px;
}

.link-item {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
}

.link-label {
  width: 120px;
  flex-shrink: 0;
}

.link-input {
  flex-grow: 1;
}

.quick-stats {
  margin: 20px 0;
}
</style>
