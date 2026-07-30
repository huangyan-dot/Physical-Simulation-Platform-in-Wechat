// pages/progress/progress.js
import { request } from '../../utils/request';
import { getCurrentRole } from '../../utils/auth';
import { MOCK_PROGRESS, MOCK_CLASSES, MOCK_CLASS_STATS } from '../../utils/mock';

const app = getApp();

Page({
  data: {
    role: 'student',
    loading: false,

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
      const res = await request({
        url: '/progress/mine',
        method: 'GET',
        showLoading: false
      });
      const data = res.data || res || {};
      this.setData({
        summary: {
          total: data.total || 0,
          passed: data.passed || 0,
          avgScore: data.avg_score || 0
        },
        records: data.records || [],
        loading: false
      });
    } catch (err) {
      console.warn('[progress] 后端不可用，使用 mock 数据');
      this.setData({
        summary: {
          total: MOCK_PROGRESS.total,
          passed: MOCK_PROGRESS.passed,
          avgScore: MOCK_PROGRESS.avg_score
        },
        records: MOCK_PROGRESS.records,
        loading: false
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
      const classes = res.data || res || [];
      this.setData({ classes, loading: false });

      if (classes.length > 0 && !this.data.selectedClassId) {
        this.setData({
          selectedClassId: classes[0].id,
          classIndex: 0
        });
        this.loadClassProgress(classes[0].id);
      }
    } catch (err) {
      console.warn('[progress] 后端不可用，使用 mock 班级');
      this.setData({
        classes: MOCK_CLASSES,
        loading: false
      });
      if (MOCK_CLASSES.length > 0 && !this.data.selectedClassId) {
        this.setData({
          selectedClassId: MOCK_CLASSES[0].id,
          classIndex: 0
        });
        this.loadClassProgress(MOCK_CLASSES[0].id);
      }
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
      const res = await request({
        url: `/progress/class/${classId}`,
        method: 'GET',
        showLoading: false
      });
      const data = res.data || res || {};
      const summary = data.summary || {};
      // 在 JS 里提前格式化，WXML 不支持 .toFixed()
      if (summary.pass_rate !== undefined) {
        summary.pass_rate_formatted = (summary.pass_rate * 100).toFixed(1);
      }
      this.setData({
        classStats: summary,
        students: data.rows || [],
        loading: false
      });
    } catch (err) {
      console.warn('[progress] 后端不可用，使用 mock 班级成绩');
      const summary = { ...MOCK_CLASS_STATS.summary };
      if (summary.pass_rate !== undefined) {
        summary.pass_rate_formatted = (summary.pass_rate * 100).toFixed(1);
      }
      this.setData({
        classStats: summary,
        students: MOCK_CLASS_STATS.rows,
        loading: false
      });
    }
  }
});
