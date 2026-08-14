// pages/assignment/publish/publish.js
import { request } from '../../../utils/request';

Page({
  data: {
    classId: null,
    className: '',
    levels: [],
    levelIndex: 0,
    selectedLevelId: null,
    title: '',
    deadline: '',
    // 综合得分比例：测量数据占 dataWeight%，自测题目占其余，默认 60:40
    dataWeight: 60,
    loading: false
  },

  onLoad(options) {
    const classId = parseInt(options.classId, 10);
    const className = decodeURIComponent(options.className || '');
    this.setData({ classId, className });
    this.loadLevels();
    // 默认截止时间：7天后
    const d = new Date(Date.now() + 7 * 86400000);
    this.setData({
      deadline: `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} 23:59`
    });
  },

  async loadLevels() {
    try {
      const res = await request({ url: '/levels', method: 'GET', showLoading: false });
      const levels = Array.isArray(res) ? res : [];
      this.setData({
        levels,
        selectedLevelId: levels.length > 0 ? levels[0].id : null
      });
    } catch (err) {
      console.error('[publish] 加载关卡失败:', err);
      wx.showToast({ title: '加载关卡失败', icon: 'none' });
    }
  },

  onLevelChange(e) {
    const index = parseInt(e.detail.value, 10);
    this.setData({ levelIndex: index, selectedLevelId: this.data.levels[index].id });
  },

  onTitleInput(e) {
    this.setData({ title: e.detail.value });
  },

  onDeadlineChange(e) {
    this.setData({ deadline: e.detail.value });
  },

  onWeightChanging(e) {
    this.setData({ dataWeight: Number(e.detail.value) });
  },

  onWeightChange(e) {
    this.setData({ dataWeight: Number(e.detail.value) });
  },

  async handlePublish() {
    const { classId, selectedLevelId, title, deadline, dataWeight } = this.data;
    if (!title.trim()) {
      wx.showToast({ title: '请输入作业标题', icon: 'none' });
      return;
    }
    if (!selectedLevelId) {
      wx.showToast({ title: '请选择实验关卡', icon: 'none' });
      return;
    }

    // 转为 ISO 8601 (RFC3339) 格式传给后端
    const iso = deadline.replace(' ', 'T') + ':59+08:00';

    this.setData({ loading: true });
    try {
      await request({
        url: `/classes/${classId}/assignments`,
        method: 'POST',
        data: {
          title: title.trim(),
          level_id: selectedLevelId,
          deadline: iso,
          data_weight: dataWeight
        }
      });
      wx.showToast({ title: '发布成功', icon: 'success' });
      setTimeout(() => wx.navigateBack(), 1500);
    } catch (err) {
      console.error('[publish] 发布失败:', err);
      wx.showToast({ title: err.message || '发布失败', icon: 'none' });
      this.setData({ loading: false });
    }
  }
});
