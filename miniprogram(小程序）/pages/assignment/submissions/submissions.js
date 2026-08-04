// pages/assignment/submissions/submissions.js
import { request } from '../../../utils/request';

Page({
  data: {
    assignmentId: null,
    title: '',
    loading: true,
    loadError: '',
    submissions: [],
    totalCount: 0,
    submittedCount: 0,
    passedCount: 0
  },

  onLoad(options) {
    const assignmentId = parseInt(options.assignmentId, 10);
    const title = decodeURIComponent(options.title || '');
    this.setData({ assignmentId, title });
    wx.setNavigationBarTitle({ title: title || '提交明细' });
    this.loadSubmissions();
  },

  async loadSubmissions() {
    this.setData({ loading: true });
    try {
      const res = await request({
        url: `/assignments/${this.data.assignmentId}/submissions`,
        method: 'GET',
        showLoading: false
      });
      const subs = Array.isArray(res) ? res : [];
      // WXML 不支持 .filter() 等数组方法，在 JS 里算好统计值
      let submitted = 0, passed = 0;
      for (const s of subs) {
        if (s.attempts > 0) submitted++;
        if (s.passed) passed++;
      }
      this.setData({
        submissions: subs,
        totalCount: subs.length,
        submittedCount: submitted,
        passedCount: passed,
        loading: false,
        loadError: ''
      });
    } catch (err) {
      console.error('[submissions] 加载失败:', err);
      this.setData({
        submissions: [],
        totalCount: 0,
        submittedCount: 0,
        passedCount: 0,
        loading: false,
        loadError: err.message || '加载失败'
      });
    }
  },

  onRetry() {
    this.loadSubmissions();
  }
});
