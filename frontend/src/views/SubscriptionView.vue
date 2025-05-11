<template>
    <div class="subscription-container">
        <div class="page-header">
            <h2>订阅管理</h2>
            <div class="header-buttons">
                <el-button type="primary" @click="refreshSubscriptionList">
                    <el-icon><Refresh /></el-icon>
                    刷新
                </el-button>
                <el-button type="primary" @click="showAddDialog">添加订阅</el-button>
            </div>
        </div>

        <el-card v-if="subscriptionStore.loading" class="loading-card">
            <el-skeleton :rows="5" animated />
        </el-card>

        <el-empty v-else-if="!subscriptionStore.subscriptions.length" description="暂无订阅，请添加" />

        <template v-else>
            <el-card v-for="subscription in subscriptionStore.subscriptions" :key="subscription.id"
                class="subscription-card">
                <div class="subscription-header">
                    <div class="subscription-title">
                        <h3>{{ subscription.name }}</h3>
                        <el-tag :type="subscription.enabled ? 'success' : 'info'">
                            {{ subscription.enabled ? '已启用' : '已禁用' }}
                        </el-tag>
                    </div>
                    <div class="subscription-actions">
                        <el-button-group>
                            <el-button size="small" @click="refreshSubscription(subscription.id!)">
                                <el-icon>
                                    <Refresh />
                                </el-icon>
                                刷新节点
                            </el-button>
                            <el-button size="small" @click="showEditDialog(subscription)">
                                <el-icon>
                                    <Edit />
                                </el-icon>
                                编辑
                            </el-button>
                            <el-button size="small" type="danger" @click="confirmDelete(subscription)">
                                <el-icon>
                                    <Delete />
                                </el-icon>
                                删除
                            </el-button>
                        </el-button-group>
                    </div>
                </div>

                <div class="subscription-info">
                    <p><strong>类型：</strong>{{ subscription.type }}</p>
                    <p><strong>URL：</strong>{{ subscription.url }}</p>
                    <p><strong>最后更新：</strong>{{ formatDate(subscription.lastUpdated) }}</p>
                    <p><strong>有效节点：</strong><el-tag size="small" type="success">{{ subscription.valid_proxy_count || 0 }}</el-tag> 个</p>
                </div>
            </el-card>
        </template>

        <!-- 添加/编辑订阅对话框 -->
        <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑订阅' : '添加订阅'" width="500px">
            <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
                <el-form-item label="名称" prop="name">
                    <el-input v-model="form.name" placeholder="请输入订阅名称" />
                </el-form-item>
                <el-form-item label="URL" prop="url">
                    <el-input v-model="form.url" placeholder="请输入订阅URL" />
                </el-form-item>
                <el-form-item label="类型" prop="type">
                    <el-select v-model="form.type" placeholder="请选择订阅类型" style="width: 100%">
                        <el-option label="自动检测" value="auto" />
                        <el-option label="V2Ray" value="v2ray" />
                        <el-option label="Shadowsocks" value="ss" />
                        <el-option label="Trojan" value="trojan" />
                    </el-select>
                </el-form-item>
                <el-form-item label="状态" prop="enabled">
                    <el-switch v-model="form.enabled" active-text="启用" inactive-text="禁用" />
                </el-form-item>
            </el-form>
            <template #footer>
                <span class="dialog-footer">
                    <el-button @click="dialogVisible = false">取消</el-button>
                    <el-button type="primary" @click="submitForm" :loading="submitting">
                        确认
                    </el-button>
                </span>
            </template>
        </el-dialog>
    </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { Refresh, Edit, Delete } from '@element-plus/icons-vue';
import { useSubscriptionStore } from '@/stores/subscription';
import { type Subscription } from '@/api';

const subscriptionStore = useSubscriptionStore();

// 表单相关
const formRef = ref<FormInstance>();
const dialogVisible = ref(false);
const isEditing = ref(false);
const submitting = ref(false);
const form = reactive({
    id: undefined as number | undefined,
    name: '',
    url: '',
    type: 'auto',
    enabled: true
});

const rules = reactive<FormRules>({
    name: [
        { required: true, message: '请输入订阅名称', trigger: 'blur' },
        { min: 2, max: 50, message: '长度在 2 到 50 个字符', trigger: 'blur' }
    ],
    url: [
        { required: true, message: '请输入订阅URL', trigger: 'blur' },
        { pattern: /^https?:\/\/.+/i, message: '请输入有效的URL', trigger: 'blur' }
    ],
    type: [
        { required: true, message: '请选择订阅类型', trigger: 'change' }
    ]
});

// 格式化日期
const formatDate = (dateString?: string) => {
    if (!dateString) return '未更新';
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN');
};

// 显示添加对话框
const showAddDialog = () => {
    isEditing.value = false;
    form.id = undefined;
    form.name = '';
    form.url = '';
    form.type = 'auto';
    form.enabled = true;
    dialogVisible.value = true;
};

// 显示编辑对话框
const showEditDialog = (subscription: Subscription) => {
    isEditing.value = true;
    form.id = subscription.id;
    form.name = subscription.name;
    form.url = subscription.url;
    form.type = subscription.type;
    form.enabled = subscription.enabled;
    dialogVisible.value = true;
};

// 提交表单
const submitForm = async () => {
    if (!formRef.value) return;

    await formRef.value.validate(async (valid) => {
        if (valid) {
            submitting.value = true;
            try {
                if (isEditing.value && form.id) {
                    await subscriptionStore.updateSubscription(form.id, {
                        name: form.name,
                        url: form.url,
                        type: form.type,
                        enabled: form.enabled
                    });
                    ElMessage.success('订阅更新成功');
                } else {
                    await subscriptionStore.addSubscription({
                        name: form.name,
                        url: form.url,
                        type: form.type,
                        enabled: form.enabled
                    });
                    ElMessage.success('订阅添加成功');
                }
                dialogVisible.value = false;
            } catch (error: any) {
                ElMessage.error(error.message || '操作失败');
            } finally {
                submitting.value = false;
            }
        }
    });
};

// 刷新订阅
const refreshSubscription = async (id: number) => {
    try {
        await subscriptionStore.refreshSubscription(id);
        ElMessage.success('订阅刷新成功');
    } catch (error: any) {
        ElMessage.error(error.message || '刷新失败');
    }
};

// 确认删除
const confirmDelete = (subscription: Subscription) => {
    ElMessageBox.confirm(
        `确定要删除订阅 "${subscription.name}" 吗？此操作将同时删除该订阅下的所有节点。`,
        '删除确认',
        {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning'
        }
    ).then(async () => {
        try {
            await subscriptionStore.deleteSubscription(subscription.id!);
            ElMessage.success('订阅删除成功');
        } catch (error: any) {
            ElMessage.error(error.message || '删除失败');
        }
    }).catch(() => {
        // 用户取消删除
    });
};

// 刷新订阅列表
const refreshSubscriptionList = async () => {
    try {
        await subscriptionStore.fetchSubscriptions();
        ElMessage.success('订阅列表刷新成功');
    } catch (error: any) {
        ElMessage.error(error.message || '刷新失败');
    }
};

// 加载数据
onMounted(() => {
    subscriptionStore.fetchSubscriptions();
});
</script>

<style scoped>
.subscription-container {
    max-width: 1000px;
    margin: 0 auto;
}

.page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
}

.header-buttons {
    display: flex;
    gap: 10px;
}

.subscription-card {
    margin-bottom: 15px;
}

.subscription-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
}

.subscription-title {
    display: flex;
    align-items: center;
    gap: 10px;
}

.subscription-title h3 {
    margin: 0;
}

.subscription-info {
    color: #606266;
    font-size: 14px;
}

.loading-card {
    padding: 20px;
    margin-bottom: 15px;
}
</style>