// pages/experiments/pendulum/pendulum.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    config: { length_m: 1.0, angle_deg: 15, gravity: 9.8 },
    periodStr: '2.01',
    readings: { period: '2.01', calc_g: '9.80' },
    submitting: false,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  _canvas: null,
  _ctx: null,
  _rafId: 0,

  onLoad(options) {
    this.setData({ levelId: options.levelId });
    this.initCanvas();
    this.updatePeriod();
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#pendulumCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[pendulum] canvas 节点获取失败');
          return;
        }
        const canvas = res[0].node;
        this._canvas = canvas;
        this._ctx = canvas.getContext('2d');
        this.startAnimation();
      });
  },

  onUnload() {
    if (this._rafId) {
      this._canvas && this._canvas.cancelAnimationFrame(this._rafId);
    }
    this._canvas = null;
    this._ctx = null;
  },

  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    const height = Math.min(width, 480);
    this.setData({
      canvasWidth: width,
      canvasHeight: height,
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  // 计算理论周期
  updatePeriod() {
    const { length_m, gravity } = this.data.config;
    const period = 2 * Math.PI * Math.sqrt(length_m / gravity);
    const periodStr = period.toFixed(2);
    this.setData({
      periodStr,
      'readings.period': periodStr,
      'readings.calc_g': gravity.toFixed(2)
    });
  },

  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;

    const loop = () => {
      this.drawPendulum();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  drawPendulum() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth, canvasHeight, pixelRatio, config } = this.data;
    const t = Date.now() / 1000;

    canvas.width = canvasWidth * pixelRatio;
    canvas.height = canvasHeight * pixelRatio;
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    const w = canvasWidth;
    const h = canvasHeight;

    // 背景
    ctx.fillStyle = '#f5f6fa';
    ctx.fillRect(0, 0, w, h);

    // 悬挂点
    const pivotX = w / 2;
    const pivotY = h * 0.15;

    // 摆长（像素）
    const maxPendulumPx = h * 0.55;
    const lengthScale = maxPendulumPx / 2.0; // 最长 2m 对应满高度
    const ropePx = config.length_m * lengthScale;

    // 角度随时间变化
    const angleRad = (config.angle_deg * Math.PI / 180);
    const omega = Math.sqrt(config.gravity / config.length_m);
    const currentAngle = angleRad * Math.cos(omega * t);

    const bobX = pivotX + Math.sin(currentAngle) * ropePx;
    const bobY = pivotY + Math.cos(currentAngle) * ropePx;

    // 绘制天花板横梁
    ctx.strokeStyle = '#7f8c8d';
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.moveTo(pivotX - 80, pivotY);
    ctx.lineTo(pivotX + 80, pivotY);
    ctx.stroke();

    // 绘制悬挂点
    ctx.fillStyle = '#2c3e50';
    ctx.beginPath();
    ctx.arc(pivotX, pivotY, 6, 0, Math.PI * 2);
    ctx.fill();

    // 绘制摆线
    ctx.strokeStyle = '#95a5a6';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(pivotX, pivotY);
    ctx.lineTo(bobX, bobY);
    ctx.stroke();

    // 绘制摆球
    const bobRadius = 18;
    const gradient = ctx.createRadialGradient(bobX - 4, bobY - 4, 2, bobX, bobY, bobRadius);
    gradient.addColorStop(0, '#e74c3c');
    gradient.addColorStop(1, '#c0392b');
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(bobX, bobY, bobRadius, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = '#96281b';
    ctx.lineWidth = 2;
    ctx.stroke();

    // 绘制虚线参考位置（平衡位置）
    ctx.setLineDash([5, 5]);
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.15)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(pivotX, pivotY);
    ctx.lineTo(pivotX, pivotY + ropePx);
    ctx.stroke();
    ctx.setLineDash([]);

    // 绘制地面
    ctx.fillStyle = '#bdc3c7';
    ctx.fillRect(0, h - 4, w, 4);
  },

  onLengthChange(e) {
    this.setData({ 'config.length_m': e.detail.value });
    this.updatePeriod();
  },

  onAngleChange(e) {
    this.setData({ 'config.angle_deg': e.detail.value });
  },

  onGravityChange(e) {
    this.setData({ 'config.gravity': e.detail.value });
    this.updatePeriod();
  },

  onReadingInput(e) {
    const { field } = e.currentTarget.dataset;
    const val = e.detail.value;
    this.setData({ [`readings.${field}`]: val });
    if (field === 'period') {
      const T = parseFloat(val);
      if (T > 0) {
        const { length_m } = this.data.config;
        const g = (4 * Math.PI * Math.PI * length_m) / (T * T);
        this.setData({ 'readings.calc_g': g.toFixed(2) });
      }
    }
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    this.setData({ submitting: true });

    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 3,
          experiment: 'pendulum',
          readings: [
            { period: parseFloat(readings.period) || 0, calc_g: parseFloat(readings.calc_g) || 0 }
          ]
        }
      });

      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => {
          if (data.passed) wx.navigateBack();
        }
      });
    } catch (err) {
      console.error('[pendulum] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});