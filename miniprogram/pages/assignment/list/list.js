// pages/assignment/list/list.js
import { request } from '../../../utils/request';
import { isRole } from '../../../utils/auth';

Page({
  data: {
    role: 'student',
    loading: true,
    loadError: '',
    // 学生
    assignments: [],
    // 教师
    classes: [],
    classIndex: 0,
    selectedClassId: null,
    classAssignments: []
  },

  onLoad() {
    this.setData({ role: isRole('teacher') || isRole('admin') ? 'teacher' : 'student' });
  },

  onShow() {
    if (typeof this.getTabBar === 'function' && this.getTabBar()) {
      this.getTabBar().setData({ selected: 2 });
    }
    if (this.data.role === 'student') {
      this.loadMine();
    } else {
      this.loadTeacherClasses();
    }
  },

  // 学生：加载我的作业
  async loadMine() {
    this.setData({ loading: true });
    try {
      const res = await request({ url: '/assignments/mine', method: 'GET', showLoading: false });
      this.setData({
        assignments: Array.isArray(res) ? res : [],
        loading: false,
        loadError: ''
      });
    } catch (err) {
      console.error('[assignment] 加载失败:', err);
      this.setData({
        assignments: [],
        loading: false,
        loadError: err.message || '加载作业失败'
      });
    }
  },

  // 学生：去实验页做作业
  goExperiment(e) {
    const { id, levelId, code, overdue } = e.currentTarget.dataset;
    if (overdue) {
      wx.showToast({ title: '作业已过截止时间', icon: 'none' });
      return;
    }
    const pageMap = {
      newton_ring: '/pages/experiments/newton-ring/newton-ring',
      oscilloscope: '/pages/experiments/oscilloscope/oscilloscope',
      pendulum: '/pages/experiments/pendulum/pendulum'
    };
    const url = pageMap[code] || pageMap.newton_ring;
    wx.navigateTo({ url: `${url}?levelId=${levelId}&assignmentId=${id}` });
  },

  // 教师：加载班级列表
  async loadTeacherClasses() {
    this.setData({ loading: true });
    try {
      const res = await request({ url: '/classes', method: 'GET', showLoading: false });
      const classes = Array.isArray(res) ? res : [];
      this.setData({ classes, loading: false, loadError: '' });
      if (classes.length > 0 && !this.data.selectedClassId) {
        this.setData({ selectedClassId: classes[0].id, classIndex: 0 });
        this.loadClassAssignments(classes[0].id);
      }
    } catch (err) {
      console.error('[assignment] 加载班级失败:', err);
      this.setData({ classes: [], loading: false, loadError: err.message || '加载班级失败' });
    }
  },

  // 教师：切换班级
  onClassChange(e) {
    const index = parseInt(e.detail.value, 10);
    const cls = this.data.classes[index];
    this.setData({ classIndex: index, selectedClassId: cls.id });
    this.loadClassAssignments(cls.id);
  },

  // 教师：加载班级作业
  async loadClassAssignments(classId) {
    this.setData({ loading: true });
    try {
      const res = await request({
        url: `/classes/${classId}/assignments`,
        method: 'GET',
        showLoading: false
      });
      this.setData({
        classAssignments: Array.isArray(res) ? res : [],
        loading: false,
        loadError: ''
      });
    } catch (err) {
      console.error('[assignment] 加载作业列表失败:', err);
      this.setData({
        classAssignments: [],
        loading: false,
        loadError: err.message || '加载作业失败'
      });
    }
  },

  // 教师：查看提交明细
  goSubmissions(e) {
    const { id, title } = e.currentTarget.dataset;
    wx.navigateTo({
      url: `/pages/assignment/submissions/submissions?assignmentId=${id}&title=${encodeURIComponent(title)}`
    });
  },

  // 教师：删除作业
  async deleteAssignment(e) {
    const { id } = e.currentTarget.dataset;
    const res = await wx.showModal({
      title: '确认删除',
      content: '删除后不可恢复，是否继续？',
      confirmColor: '#e74c3c'
    });
    if (!res.confirm) return;

    try {
      await request({ url: `/assignments/${id}`, method: 'DELETE' });
      wx.showToast({ title: '删除成功', icon: 'success' });
      if (this.data.selectedClassId) {
        this.loadClassAssignments(this.data.selectedClassId);
      }
    } catch (err) {
      wx.showToast({ title: err.message || '删除失败', icon: 'none' });
    }
  },

  onRetry() {
    if (this.data.role === 'student') {
      this.loadMine();
    } else {
      this.loadTeacherClasses();
    }
  }
});
