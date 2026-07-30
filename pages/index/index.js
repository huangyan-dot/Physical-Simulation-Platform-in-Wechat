// pages/index/index.js
import { request } from '../../utils/request';
import { checkLogin } from '../../utils/auth';
import { MOCK_LEVELS, MOCK_PROGRESS } from '../../utils/mock';

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
      console.warn('[index] 后端不可用，使用 mock 数据');
      this.setData({
        levels: this.formatLevels(MOCK_LEVELS),
        loading: false
      });
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
      console.warn('[index] 后端不可用，使用 mock 进度');
      this.setData({
        progress: {
          total: MOCK_PROGRESS.total,
          passed: MOCK_PROGRESS.passed,
          bestScore: MOCK_PROGRESS.best_score
        }
      });
    }
  },

  // 格式化关卡数据（补充图标、描述等）
  formatLevels(levels) {
    const experimentMeta = {
      newton_ring: { icon: '🔬', description: '用牛顿环测量透镜曲率半径' },
      oscilloscope: { icon: '〰️', description: '观测正弦波与李萨如图形' },
      pendulum: { icon: '📐', description: '研究单摆周期与重力加速度' }
    };
    return levels.map(item => {
      const diff = item.difficulty || 1;
      return {
        ...item,
        icon: experimentMeta[item.experiment_code]?.icon || '🧪',
        description: item.description || experimentMeta[item.experiment_code]?.description || '物理实验模拟',
        difficultyStars: '⭐'.repeat(diff)
      };
    });
  },

  // 点击关卡
  onLevelTap(e) {
    const { id, status, experiment } = e.currentTarget.dataset;

    if (status === 'locked') {
      wx.showToast({ title: '请先通过前置关卡', icon: 'none' });
      return;
    }

    const pageMap = {
      newton_ring: '/pages/experiments/newton-ring/newton-ring',
      oscilloscope: '/pages/experiments/oscilloscope/oscilloscope',
      pendulum: '/pages/experiments/pendulum/pendulum'
    };

    const url = pageMap[experiment] || '/pages/experiments/newton-ring/newton-ring';
    wx.navigateTo({ url: `${url}?levelId=${id}` });
  }
});
