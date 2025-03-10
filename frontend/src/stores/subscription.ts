import { defineStore } from 'pinia';
import { subscriptionApi, type Subscription } from '@/api';

export const useSubscriptionStore = defineStore('subscription', {
  state: () => ({
    subscriptions: [] as Subscription[],
    loading: false,
    error: null as string | null,
  }),
  
  getters: {
    getSubscriptionById: (state) => (id: number) => {
      return state.subscriptions.find(sub => sub.id === id);
    },
    enabledSubscriptions: (state) => {
      return state.subscriptions.filter(sub => sub.enabled);
    },
  },
  
  actions: {
    async fetchSubscriptions() {
      this.loading = true;
      this.error = null;
      
      try {
        const response = await subscriptionApi.getAll();
        this.subscriptions = response.data;
      } catch (error: any) {
        this.error = error.message || '获取订阅失败';
        console.error('获取订阅失败:', error);
      } finally {
        this.loading = false;
      }
    },
    
    async addSubscription(subscription: Subscription) {
      this.loading = true;
      this.error = null;
      
      try {
        const response = await subscriptionApi.create(subscription);
        this.subscriptions.push(response.data);
        return response.data;
      } catch (error: any) {
        this.error = error.message || '添加订阅失败';
        console.error('添加订阅失败:', error);
        throw error;
      } finally {
        this.loading = false;
      }
    },
    
    async updateSubscription(id: number, subscription: Subscription) {
      this.loading = true;
      this.error = null;
      
      try {
        const response = await subscriptionApi.update(id, subscription);
        const index = this.subscriptions.findIndex(sub => sub.id === id);
        if (index !== -1) {
          this.subscriptions[index] = response.data;
        }
        return response.data;
      } catch (error: any) {
        this.error = error.message || '更新订阅失败';
        console.error('更新订阅失败:', error);
        throw error;
      } finally {
        this.loading = false;
      }
    },
    
    async deleteSubscription(id: number) {
      this.loading = true;
      this.error = null;
      
      try {
        await subscriptionApi.delete(id);
        const index = this.subscriptions.findIndex(sub => sub.id === id);
        if (index !== -1) {
          this.subscriptions.splice(index, 1);
        }
      } catch (error: any) {
        this.error = error.message || '删除订阅失败';
        console.error('删除订阅失败:', error);
        throw error;
      } finally {
        this.loading = false;
      }
    },
    
    async refreshSubscription(id: number) {
      this.loading = true;
      this.error = null;
      
      try {
        const response = await subscriptionApi.refresh(id);
        const index = this.subscriptions.findIndex(sub => sub.id === id);
        if (index !== -1) {
          this.subscriptions[index] = response.data.subscription;
        }
        return response.data;
      } catch (error: any) {
        this.error = error.message || '刷新订阅失败';
        console.error('刷新订阅失败:', error);
        throw error;
      } finally {
        this.loading = false;
      }
    },
  },
});