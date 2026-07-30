// pages/class/class.js
import { request } from '../../utils/request';
import { isRole } from '../../utils/auth';
import { MOCK_CLASSES } from '../../utils/mock';

Page({
  data: {
    classes: [],
    loading: false,
    loadingList: true,
    canManage: false,
    newClassName: '',
    showAddModal: false,
    selectedClassId: null,
    addUserId: ''
  },

  onLoad() {
    this.setData({
      canManage: isRole('teacher') || isRole('admin')
    });
  },

  onShow() {
    if (typeof this.getTabBar === 'function' && this.getTabBar()) {
      this.getTabBar().setData({ selected: 1 });
    }
    this.loadClasses();
  },

  // 加载班级列表
  async loadClasses() {
    this.setData({ loadingList: true });
    try {
      const res = await request({
        url: '/classes',
        method: 'GET',
        showLoading: false
      });
      this.setData({
        classes: res.data || res || [],
        loadingList: false
      });
    } catch (err) {
      console.warn('[class] 后端不可用，使用 mock 数据');
      this.setData({
        classes: MOCK_CLASSES,
        loadingList: false
      });
    }
  },

  // 输入班级名称
  onClassNameInput(e) {
    this.setData({ newClassName: e.detail.value });
  },

  // 创建班级
  async createClass() {
    const { newClassName } = this.data;
    if (!newClassName.trim()) {
      wx.showToast({ title: '请输入班级名称', icon: 'none' });
      return;
    }

    this.setData({ loading: true });
    try {
      await request({
        url: '/classes',
        method: 'POST',
        data: { name: newClassName.trim() }
      });
      wx.showToast({ title: '创建成功', icon: 'success' });
      this.setData({ newClassName: '', loading: false });
      this.loadClasses();
    } catch (err) {
      console.error('[class] 创建班级失败:', err);
      wx.showToast({ title: err.message || '创建失败', icon: 'none' });
      this.setData({ loading: false });
    }
  },

  // 删除班级
  async deleteClass(e) {
    const { id } = e.currentTarget.dataset;
    const res = await wx.showModal({
      title: '确认删除',
      content: '删除后不可恢复，是否继续？',
      confirmColor: '#e74c3c'
    });
    if (!res.confirm) return;

    try {
      await request({
        url: `/classes/${id}`,
        method: 'DELETE'
      });
      wx.showToast({ title: '删除成功', icon: 'success' });
      this.loadClasses();
    } catch (err) {
      console.error('[class] 删除班级失败:', err);
      wx.showToast({ title: err.message || '删除失败', icon: 'none' });
    }
  },

  // 显示添加成员弹窗
  showAddMember(e) {
    const { id } = e.currentTarget.dataset;
    this.setData({
      showAddModal: true,
      selectedClassId: id,
      addUserId: ''
    });
  },

  // 关闭弹窗
  closeModal() {
    this.setData({ showAddModal: false });
  },

  preventBubble() {
    // 阻止冒泡
  },

  onAddUserIdInput(e) {
    this.setData({ addUserId: e.detail.value });
  },

  // 添加成员
  async addMember() {
    const { selectedClassId, addUserId } = this.data;
    if (!addUserId.trim()) {
      wx.showToast({ title: '请输入学生 ID', icon: 'none' });
      return;
    }

    this.setData({ loading: true });
    try {
      await request({
        url: `/classes/${selectedClassId}/members`,
        method: 'POST',
        data: { user_id: parseInt(addUserId.trim(), 10) }
      });
      wx.showToast({ title: '添加成功', icon: 'success' });
      this.setData({ showAddModal: false, loading: false });
      this.loadClasses();
    } catch (err) {
      console.error('[class] 添加成员失败:', err);
      wx.showToast({ title: err.message || '添加失败', icon: 'none' });
      this.setData({ loading: false });
    }
  }
});
