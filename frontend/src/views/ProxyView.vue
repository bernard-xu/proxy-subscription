<template>
    <div class="proxy-container">
        <div class="page-header">
            <h2>节点列表</h2>
            <div class="filter-actions">
                <el-select v-model="selectedSubscription" placeholder="选择订阅过滤" clearable
                    @change="handleSubscriptionChange">
                    <el-option v-for="sub in subscriptionStore.subscriptions" :key="sub.id!" :label="sub.name"
                        :value="sub.id" />
                </el-select>
                <el-input v-model="searchQuery" placeholder="搜索节点" clearable style="width: 200px; margin-left: 10px;">
                    <template #prefix>
                        <el-icon>
                            <Search />
                        </el-icon>
                    </template>
                </el-input>
            </div>
        </div>

        <el-card v-if="proxyStore.loading" class="loading-card">
            <el-skeleton :rows="10" animated />
        </el-card>

        <el-empty v-else-if="!filteredProxies.length" description="暂无节点" />

        <el-table v-else :data="filteredProxies" style="width: 100%" border stripe>
            <el-table-column prop="name" label="名称" min-width="200" show-overflow-tooltip />
            <el-table-column prop="type" label="类型" width="100">
                <template #default="scope">
                    <el-tag :type="getProxyTypeTag(scope.row.type)">
                        {{ scope.row.type.toUpperCase() }}
                    </el-tag>
                </template>
            </el-table-column>
            <el-table-column prop="server" label="服务器" min-width="150" show-overflow-tooltip />
            <el-table-column prop="port" label="端口" width="100" />
            <el-table-column prop="subscription_name" label="订阅来源" min-width="150" />
            <el-table-column label="操作" width="150" fixed="right">
                <template #default="scope">
                    <el-button-group>
                        <el-button size="small" @click="showProxyDetail(scope.row)">
                            <el-icon>
                                <View />
                            </el-icon>
                            详情
                        </el-button>
                    </el-button-group>
                </template>
            </el-table-column>
        </el-table>

        <!-- 节点详情对话框 -->
        <el-dialog v-model="detailDialogVisible" title="节点详情" width="600px">
            <div v-if="selectedProxy">
                <el-descriptions :column="1" border>
                    <el-descriptions-item label="名称">{{ selectedProxy.name }}</el-descriptions-item>
                    <el-descriptions-item label="类型">{{ selectedProxy.type.toUpperCase() }}</el-descriptions-item>
                    <el-descriptions-item label="服务器">{{ selectedProxy.server }}</el-descriptions-item>
                    <el-descriptions-item label="端口">{{ selectedProxy.port }}</el-descriptions-item>

                    <template v-if="selectedProxy.uuid">
                        <el-descriptions-item label="UUID">{{ selectedProxy.uuid }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.password">
                        <el-descriptions-item label="密码">{{ selectedProxy.password }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.method">
                        <el-descriptions-item label="加密方式">{{ selectedProxy.method }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.network">
                        <el-descriptions-item label="传输协议">{{ selectedProxy.network }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.path">
                        <el-descriptions-item label="路径">{{ selectedProxy.path }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.host">
                        <el-descriptions-item label="主机名">{{ selectedProxy.host }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.tls !== undefined">
                        <el-descriptions-item label="TLS">{{ selectedProxy.tls ? '启用' : '禁用' }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.sni">
                        <el-descriptions-item label="SNI">{{ selectedProxy.sni }}</el-descriptions-item>
                    </template>

                    <template v-if="selectedProxy.alpn">
                        <el-descriptions-item label="ALPN">{{ selectedProxy.alpn }}</el-descriptions-item>
                    </template>
                </el-descriptions>

                <div class="raw-config" v-if="selectedProxy.rawConfig">
                    <h4>原始配置</h4>
                    <el-input type="textarea" :rows="5" v-model="selectedProxy.rawConfig" readonly />
                </div>
            </div>
        </el-dialog>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { Search, View } from '@element-plus/icons-vue';
import { useSubscriptionStore } from '@/stores/subscription';
import { useProxyStore } from '@/stores/proxy';
import { type Proxy } from '@/api';

const subscriptionStore = useSubscriptionStore();
const proxyStore = useProxyStore();

// 过滤和搜索
const selectedSubscription = ref<number | null>(null);
const searchQuery = ref('');

// 节点详情
const detailDialogVisible = ref(false);
const selectedProxy = ref<Proxy | null>(null);

// 过滤后的代理节点列表
const filteredProxies = computed(() => {
    let result = proxyStore.proxies;

    // 按订阅过滤
    if (selectedSubscription.value) {
        result = result.filter(proxy => proxy.subscription_id === selectedSubscription.value);
    }

    // 按搜索关键词过滤
    if (searchQuery.value) {
        const query = searchQuery.value.toLowerCase();
        result = result.filter(proxy =>
            proxy.name.toLowerCase().includes(query) ||
            proxy.server.toLowerCase().includes(query)
        );
    }

    return result;
});

// 获取订阅名称
const getSubscriptionName = (subscriptionId: number) => {
    const subscription = subscriptionStore.getSubscriptionById(subscriptionId);
    return subscription ? subscription.name : '未知订阅';
};

// 获取代理类型对应的标签类型
const getProxyTypeTag = (type: string) => {
    switch (type.toLowerCase()) {
        case 'vmess':
            return 'primary';
        case 'ss':
            return 'success';
        case 'trojan':
            return 'warning';
        default:
            return 'info';
    }
};

// 显示节点详情
const showProxyDetail = (proxy: Proxy) => {
    selectedProxy.value = proxy;
    detailDialogVisible.value = true;
};

// 处理订阅选择变化
const handleSubscriptionChange = () => {
    // 如果需要，可以在这里添加额外的处理逻辑
};

// 加载数据
onMounted(async () => {
    await subscriptionStore.fetchSubscriptions();
    await proxyStore.fetchProxies();
});

// 监听订阅变化，重新加载节点
watch(() => subscriptionStore.subscriptions, () => {
    proxyStore.fetchProxies();
}, { deep: true });
</script>

<style scoped>
.proxy-container {
    max-width: 1200px;
    margin: 0 auto;
}

.page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
}

.filter-actions {
    display: flex;
    align-items: center;
}

.loading-card {
    padding: 20px;
    margin-bottom: 15px;
}

.raw-config {
    margin-top: 20px;
}

.raw-config h4 {
    margin-bottom: 10px;
}
</style>