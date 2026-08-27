// pages/index/index.js
import { request } from '../../utils/request';
import { checkLogin } from '../../utils/auth';

const app = getApp();

Page({
  data: {
    userInfo: {},
    roleText: {
      student: '学生',
      teacher: '教师',
      admin: '管理员'
    },
    loading: true,
    levels: [],
    progress: {
      total: 0,
      passed: 0,
      bestScore: 0
    }
  },

  onLoad() {
    if (!checkLogin()) return;
    this.setData({ userInfo: app.globalData.userInfo || {} });
  },

  onShow() {
    if (typeof this.getTabBar === 'function' && this.getTabBar()) {
      this.getTabBar().setData({ selected: 0 });
    }
    if (app.globalData.isLogin) {
      this.loadLevels();
      this.loadProgress();
    }
  },

  onPullDownRefresh() {
    Promise.all([this.loadLevels(), this.loadProgress()])
      .finally(() => wx.stopPullDownRefresh());
  },

  // 加载关卡列表
  async loadLevels() {
    this.setData({ loading: true });
    try {
      const res = await request({
        url: '/levels',
        method: 'GET',
        showLoading: false
      });
      this.setData({
        levels: this.formatLevels(res.data || res || []),
        loading: false
      });
    } catch (err) {
      console.error('[index] 关卡列表加载失败:', err);
      this.setData({ levels: [], loading: false });
      wx.showToast({ title: '关卡加载失败，请确认后端已启动', icon: 'none' });
    }
  },

  // 加载个人进度
  async loadProgress() {
    try {
      const res = await request({
        url: '/progress/mine',
        method: 'GET',
        showLoading: false
      });
      const data = res.data || res || {};
      this.setData({
        progress: {
          total: data.total || 0,
          passed: data.passed || 0,
          bestScore: data.best_score || 0
        }
      });
    } catch (err) {
      console.error('[index] 进度加载失败:', err);
    }
  },

  // 格式化关卡数据（补充图标、描述等）
  formatLevels(levels) {
    // 与后端 experiments 表（migration 006/007/008）一一对应，键为 experiment_code
    const experimentMeta = {
      newton_ring: { icon: '💫', description: '牛顿环干涉测透镜曲率半径' },
      oscilloscope: { icon: '📟', description: '示波器测量信号幅度与频率' },
      bisection: { icon: '✂️', description: '区间对分逐步逼近方程的根' },
      newton_iter: { icon: '🎯', description: '切线迭代快速求方程的根' }
    };
    // 解锁链只约束学生：教师/管理员不受前置限制，全部关卡可直接进入预览
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    const isStaff = role === 'teacher' || role === 'admin';
    return levels.map(item => {
      const diff = item.difficulty || 1;
      return {
        ...item,
        status: isStaff && item.status === 'locked' ? 'unlocked' : item.status,
        isPreview: isStaff,
        icon: experimentMeta[item.experiment_code]?.icon || '🧪',
        description: item.description || experimentMeta[item.experiment_code]?.description || '数值实验模拟',
        difficultyStars: '⭐'.repeat(diff)
      };
    });
  },

  // 点击关卡
  onLevelTap(e) {
    const { id, status, experiment } = e.currentTarget.dataset;
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    const isStaff = role === 'teacher' || role === 'admin';

    // 解锁链只约束学生：教师/管理员可直接进入任意关卡预览
    if (status === 'locked' && !isStaff) {
      wx.showToast({ title: '请先通过前置关卡', icon: 'none' });
      return;
    }

    // 页面路由：experiment_code -> 实验页路径（与后端 experiments.code 对应）
    const pageMap = {
      newton_ring: '/pages/experiments/newton-ring/newton-ring',
      oscilloscope: '/pages/experiments/oscilloscope/oscilloscope',
      bisection: '/pages/experiments/bisection/bisection',
      newton_iter: '/pages/experiments/newton-iter/newton-iter'
    };

    const url = pageMap[experiment];
    if (!url) {
      wx.showToast({ title: '该实验页面未注册', icon: 'none' });
      return;
    }
    wx.navigateTo({ url: `${url}?levelId=${id}` });
  }
});
