// pages/class/class.js
import { request } from '../../utils/request';
import { isRole } from '../../utils/auth';

Page({
  data: {
    classes: [],
    loading: false,
    loadingList: true,
    loadError: '',
    canManage: false,
    newClassName: '',
    showAddModal: false,
    selectedClassId: null,
    addUserId: '',
    // 学生加入班级
    showJoinModal: false,
    joinCode: '',
    // 教师查看成员
    showMembersModal: false,
    members: []
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
        classes: Array.isArray(res) ? res : [],
        loadingList: false,
        loadError: ''
      });
    } catch (err) {
      console.error('[class] 加载班级失败:', err);
      this.setData({
        classes: [],
        loadingList: false,
        loadError: err.message || '加载班级失败'
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

  // 复制班级码
  copyCode(e) {
    const { code } = e.currentTarget.dataset;
    wx.setClipboardData({
      data: code,
      success() {
        wx.showToast({ title: '班级码已复制', icon: 'success' });
      }
    });
  },

  // ===== 学生：加入班级 =====
  showJoinModal() {
    this.setData({ showJoinModal: true, joinCode: '' });
  },

  onJoinCodeInput(e) {
    this.setData({ joinCode: e.detail.value.trim().toUpperCase() });
  },

  async joinClass() {
    const { joinCode } = this.data;
    if (!joinCode) {
      wx.showToast({ title: '请输入班级码', icon: 'none' });
      return;
    }

    this.setData({ loading: true });
    try {
      await request({
        url: '/classes/join',
        method: 'POST',
        data: { code: joinCode }
      });
      wx.showToast({ title: '加入成功', icon: 'success' });
      this.setData({ showJoinModal: false, loading: false });
      this.loadClasses();
    } catch (err) {
      console.error('[class] 加入班级失败:', err);
      wx.showToast({ title: err.message || '加入失败', icon: 'none' });
      this.setData({ loading: false });
    }
  },

  // ===== 教师：查看成员 =====
  async showMembers(e) {
    const { id } = e.currentTarget.dataset;
    this.setData({ showMembersModal: true, members: [] });
    try {
      const res = await request({
        url: `/classes/${id}/members`,
        method: 'GET',
        showLoading: false
      });
      this.setData({ members: Array.isArray(res) ? res : [] });
    } catch (err) {
      console.error('[class] 加载成员失败:', err);
      wx.showToast({ title: err.message || '加载成员失败', icon: 'none' });
    }
  },

  // ===== 教师：添加成员 =====
  showAddMember(e) {
    const { id } = e.currentTarget.dataset;
    this.setData({
      showAddModal: true,
      selectedClassId: id,
      addUserId: ''
    });
  },

  closeModal() {
    this.setData({ showAddModal: false, showJoinModal: false, showMembersModal: false });
  },

  preventBubble() {
    // 阻止冒泡
  },

  onAddUserIdInput(e) {
    this.setData({ addUserId: e.detail.value });
  },

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
  },

  // 教师：发布作业
  goPublishAssignment(e) {
    const { id, name } = e.currentTarget.dataset;
    wx.navigateTo({ url: `/pages/assignment/publish/publish?classId=${id}&className=${encodeURIComponent(name)}` });
  }
});
