import { defineStore } from 'pinia';
import { proxyApi, type Proxy } from '@/api';

export const useProxyStore = defineStore('proxy', {
  state: () => ({
    proxies: [] as Proxy[],
    loading: false,
    error: null as string | null,
  }),
  
  getters: {
    getProxyById: (state) => (id: number) => {
      return state.proxies.find(proxy => proxy.id === id);
    },
    getProxiesBySubscription: (state) => (subscriptionId: number) => {
      return state.proxies.filter(proxy => proxy.subscription_id === subscriptionId);
    },
  },
  
  actions: {
    async fetchProxies(subscriptionId?: number) {
      this.loading = true;
      this.error = null;
      
      try {
        const response = await proxyApi.getAll(subscriptionId);
        this.proxies = response.data;
      } catch (error: any) {
        this.error = error.message || '获取代理节点失败';
        console.error('获取代理节点失败:', error);
      } finally {
        this.loading = false;
      }
    },
    
    async fetchProxyById(id: number) {
      this.loading = true;
      this.error = null;
      
      try {
        const response = await proxyApi.getById(id);
        const index = this.proxies.findIndex(proxy => proxy.id === id);
        if (index !== -1) {
          this.proxies[index] = response.data;
        } else {
          this.proxies.push(response.data);
        }
        return response.data;
      } catch (error: any) {
        this.error = error.message || '获取代理节点详情失败';
        console.error('获取代理节点详情失败:', error);
      } finally {
        this.loading = false;
      }
    },
  },
});