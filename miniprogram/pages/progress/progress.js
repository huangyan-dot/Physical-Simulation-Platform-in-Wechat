// pages/progress/progress.js
import { request } from '../../utils/request';
import { getCurrentRole } from '../../utils/auth';

const app = getApp();

Page({
  data: {
    role: 'student',
    loading: false,
    loadError: '',

    // 学生
    summary: { total: 0, passed: 0, avgScore: 0 },
    records: [],
    statusText: {
      locked: '未解锁',
      unlocked: '未开始',
      in_progress: '进行中',
      passed: '已通过'
    },

    // 教师
    classes: [],
    classIndex: 0,
    selectedClassId: null,
    classStats: {},
    students: []
  },

  onLoad() {
    const role = getCurrentRole() || 'student';
    this.setData({ role });
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

  // 学生：加载个人进度
  async loadMine() {
    this.setData({ loading: true });
    try {
      const data = await request({
        url: '/progress/mine',
        method: 'GET',
        showLoading: false
      });
      this.setData({
        summary: {
          total: data.total || 0,
          passed: data.passed || 0,
          avgScore: data.avg_score || 0
        },
        records: data.records || [],
        loading: false,
        loadError: ''
      });
    } catch (err) {
      console.error('[progress] 加载个人进度失败:', err);
      this.setData({
        summary: { total: 0, passed: 0, avgScore: 0 },
        records: [],
        loading: false,
        loadError: err.message || '加载进度失败'
      });
    }
  },

  // 教师：加载班级列表
  async loadTeacherClasses() {
    this.setData({ loading: true });
    try {
      const res = await request({
        url: '/classes',
        method: 'GET',
        showLoading: false
      });
      const classes = Array.isArray(res) ? res : [];
      this.setData({ classes, loading: false, loadError: '' });

      if (classes.length > 0 && !this.data.selectedClassId) {
        this.setData({
          selectedClassId: classes[0].id,
          classIndex: 0
        });
        this.loadClassProgress(classes[0].id);
      }
    } catch (err) {
      console.error('[progress] 加载班级列表失败:', err);
      this.setData({
        classes: [],
        students: [],
        classStats: {},
        loading: false,
        loadError: err.message || '加载班级失败'
      });
    }
  },

  // 教师：选择班级
  onClassChange(e) {
    const index = parseInt(e.detail.value, 10);
    const selectedClass = this.data.classes[index];
    this.setData({
      classIndex: index,
      selectedClassId: selectedClass.id
    });
    this.loadClassProgress(selectedClass.id);
  },

  // 教师：加载班级成绩
  async loadClassProgress(classId) {
    this.setData({ loading: true });
    try {
      const data = await request({
        url: `/progress/class/${classId}`,
        method: 'GET',
        showLoading: false
      });
      const summary = { ...(data.summary || {}) };
      // 在 JS 里提前格式化，WXML 不支持 .toFixed()
      if (summary.pass_rate !== undefined) {
        summary.pass_rate_formatted = (summary.pass_rate * 100).toFixed(1);
      }
      this.setData({
        classStats: summary,
        students: data.rows || [],
        loading: false,
        loadError: ''
      });
    } catch (err) {
      console.error('[progress] 加载班级成绩失败:', err);
      this.setData({
        classStats: {},
        students: [],
        loading: false,
        loadError: err.message || '加载班级成绩失败'
      });
    }
  },

  // 重试
  onRetry() {
    if (this.data.role === 'student') {
      this.loadMine();
    } else {
      this.loadTeacherClasses();
    }
  }
});
