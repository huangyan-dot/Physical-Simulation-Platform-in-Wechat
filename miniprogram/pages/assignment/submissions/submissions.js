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
    passedCount: 0,
    avgCombo: 0,
    // 综合得分比例（教师可调，默认 测量60 : 自测40）
    dataWeight: 60,
    quizWeight: 40,
    // 滑杆的临时值，点保存才提交
    draftWeight: 60,
    savingWeight: false
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
      const rows = Array.isArray(res.rows) ? res.rows : [];
      const dataWeight = res.data_weight > 0 ? res.data_weight : 60;

      // WXML 不支持 .filter() 等数组方法，在 JS 里算好统计值
      let submitted = 0, passed = 0, comboSum = 0;
      for (const s of rows) {
        if (s.attempts > 0) {
          submitted++;
          comboSum += s.combo_score || 0;
        }
        if (s.passed) passed++;
      }

      this.setData({
        submissions: rows,
        totalCount: rows.length,
        submittedCount: submitted,
        passedCount: passed,
        avgCombo: submitted > 0 ? Math.round(comboSum / submitted) : 0,
        dataWeight,
        quizWeight: 100 - dataWeight,
        draftWeight: dataWeight,
        title: res.title || this.data.title,
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
        avgCombo: 0,
        loading: false,
        loadError: err.message || '加载失败'
      });
    }
  },

  // 拖动滑杆只改草稿值，实时显示比例，不立即请求
  onWeightChanging(e) {
    this.setData({ draftWeight: Number(e.detail.value) });
  },

  onWeightChange(e) {
    this.setData({ draftWeight: Number(e.detail.value) });
  },

  resetWeight() {
    this.setData({ draftWeight: 60 });
  },

  async saveWeight() {
    const { assignmentId, draftWeight } = this.data;
    this.setData({ savingWeight: true });
    try {
      await request({
        url: `/assignments/${assignmentId}/weight`,
        method: 'PATCH',
        data: { data_weight: draftWeight }
      });
      wx.showToast({ title: '比例已保存', icon: 'success' });
      // 重新拉取：后端会按新比例重算每人的综合分
      await this.loadSubmissions();
    } catch (err) {
      console.error('[submissions] 保存比例失败:', err);
      wx.showToast({ title: err.message || '保存失败', icon: 'none' });
    } finally {
      this.setData({ savingWeight: false });
    }
  },

  onRetry() {
    this.loadSubmissions();
  }
});
