// pages/experiments/young-modulus/young-modulus.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    config: {
      mass_kg: 0.5,        // 砝码质量 kg
      length_m: 1.0,       // 金属丝原长 m
      diameter_mm: 0.5,    // 金属丝直径 mm
      lever_arm_mm: 70,    // 光杠杆臂长 mm
      mirror_dist_m: 1.5   // 镜尺距离 m
    },
    youngModulusStr: '2.00×10¹¹',
    readings: [],
    submitting: false,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  _canvas: null,
  _ctx: null,
  _rafId: 0,

  onLoad(options) {
    const levelId = options.levelId;
    this.setData({ levelId });
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.generateReadings();
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#ymCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[young-modulus] canvas 节点获取失败');
          return;
        }
        const canvas = res[0].node;
        const ctx = canvas.getContext('2d');
        this._canvas = canvas;
        this._ctx = ctx;
        this.startAnimation();
      });
  },

  onUnload() {
    if (this._rafId && this._canvas) {
      this._canvas.cancelAnimationFrame(this._rafId);
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

  async loadExperimentConfig(levelId) {
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      const data = res.data || res || {};
      if (data.config) {
        const c = data.config;
        // 后端存 SI 单位，前端展示用常用单位
        const config = {
          mass_kg: c.force_n ? (c.force_n / 9.81).toFixed(2) * 1 : 0.5,
          length_m: c.length_m || 1.0,
          diameter_mm: (c.diameter_m || 0.0005) * 1000,
          lever_arm_mm: (c.lever_arm_m || 0.070) * 1000,
          mirror_dist_m: c.mirror_dist_m || 1.5
        };
        this.setData({ config });
      }
    } catch (err) {
      console.warn('[young-modulus] 后端不可用，使用默认配置');
    }
    this.generateReadings();
  },

  // 生成理论读数表
  generateReadings() {
    const { config } = this.data;
    const E = 2.0e11; // 钢丝杨氏模量标准值
    const F = config.mass_kg * 9.81; // N
    const L = config.length_m;
    const d = config.diameter_mm * 1e-3; // mm -> m
    const b = config.lever_arm_mm * 1e-3;
    const D = config.mirror_dist_m;

    const readings = [];
    const baseReading = 5.000; // mm，望远镜初始读数
    for (let i = 0; i <= 5; i++) {
      // 光杠杆放大后伸长 Δn = (8·F·i·L·D) / (π·d²·b·E) (m) -> mm
      const deltaM = (8 * F * i * L * D) / (Math.PI * d * d * b * E);
      const deltaMM = deltaM * 1e3;
      const reading = (baseReading + deltaMM).toFixed(3);
      const descending = (baseReading + deltaMM * 0.98).toFixed(3); // 减荷略低
      readings.push({ load: i, reading, descending });
    }
    this.setData({ readings });
  },

  // 持续重绘
  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => {
      this.drawScene();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  // 绘制光杠杆测量示意图
  drawScene() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth, canvasHeight, pixelRatio, config } = this.data;
    canvas.width = canvasWidth * pixelRatio;
    canvas.height = canvasHeight * pixelRatio;
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    const w = canvasWidth;
    const h = canvasHeight;

    // 背景
    ctx.fillStyle = '#1a1a2e';
    ctx.fillRect(0, 0, w, h);

    // 绘制支架
    const支架X = w * 0.15;
    const支架Top = h * 0.1;
    const支架Bottom = h * 0.85;

    ctx.strokeStyle = '#7f8c8d';
    ctx.lineWidth = 3;
    // 左立柱
    ctx.beginPath();
    ctx.moveTo(支架X, 支架Top);
    ctx.lineTo(支架X, 支架Bottom);
    ctx.stroke();
    // 底座
    ctx.beginPath();
    ctx.moveTo(支架X - 30, 支架Bottom);
    ctx.lineTo(支架X + 30, 支架Bottom);
    ctx.stroke();
    // 横梁
    ctx.beginPath();
    ctx.moveTo(支架X - 10, 支架Top);
    ctx.lineTo(w * 0.5, 支架Top);
    ctx.stroke();

    // 金属丝
    const wireX = w * 0.5;
    const wireTop = 支架Top;
    // 伸长量（像素），与砝码数成正比
    const F = config.mass_kg * 9.81;
    const E = 2.0e11;
    const L = config.length_m;
    const d = config.diameter_mm * 1e-3;
    const b = config.lever_arm_mm * 1e-3;
    const D = config.mirror_dist_m;
    const deltaM = (8 * F * 5 * L * D) / (Math.PI * d * d * b * E); // 最大伸长(5个砝码)
    const stretchPx = Math.min(deltaM * 1e3 * 50, h * 0.15); // 放大显示
    const wireBottom = h * 0.55 + stretchPx;

    ctx.strokeStyle = '#e74c3c';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(wireX, wireTop);
    ctx.lineTo(wireX, wireBottom);
    ctx.stroke();

    // 砝码
    const weightW = 40;
    const weightH = 20;
    ctx.fillStyle = '#34495e';
    ctx.fillRect(wireX - weightW / 2, wireBottom, weightW, weightH);
    ctx.strokeStyle = '#2c3e50';
    ctx.lineWidth = 1;
    ctx.strokeRect(wireX - weightW / 2, wireBottom, weightW, weightH);
    ctx.fillStyle = '#ecf0f1';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText(config.mass_kg + 'kg', wireX, wireBottom + 14);

    // 光杠杆（小镜片）
    const mirrorX = wireX + 15;
    const mirrorY = wireBottom - 10;
    ctx.fillStyle = '#f1c40f';
    ctx.fillRect(mirrorX, mirrorY - 15, 4, 30);
    // 光杠杆后足
    ctx.strokeStyle = '#f1c40f';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(mirrorX + 2, mirrorY - 15);
    ctx.lineTo(mirrorX + 2 + config.lever_arm_mm * 0.3, mirrorY - 15 - 8);
    ctx.stroke();

    // 望远镜和标尺
    const scopeX = w * 0.82;
    const scopeY = h * 0.35;
    ctx.fillStyle = '#3498db';
    ctx.fillRect(scopeX - 8, scopeY - 20, 16, 40);
    ctx.strokeStyle = '#2980b9';
    ctx.lineWidth = 1;
    ctx.strokeRect(scopeX - 8, scopeY - 20, 16, 40);

    // 标尺
    ctx.strokeStyle = '#ecf0f1';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(scopeX, scopeY - 30);
    ctx.lineTo(scopeX, scopeY + 30);
    ctx.stroke();
    for (let i = -3; i <= 3; i++) {
      const tickY = scopeY + i * 10;
      ctx.beginPath();
      ctx.moveTo(scopeX - 5, tickY);
      ctx.lineTo(scopeX, tickY);
      ctx.stroke();
    }

    // 光路（镜片 -> 望远镜）
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.4)';
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(mirrorX + 2, mirrorY);
    ctx.lineTo(scopeX - 8, scopeY);
    ctx.stroke();
    ctx.setLineDash([]);

    // 标注
    ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText('支架', 支架X - 25, 支架Bottom + 15);
    ctx.fillText('金属丝', wireX + 8, (wireTop + wireBottom) / 2);
    ctx.fillText('光杠杆', mirrorX + 10, mirrorY - 20);
    ctx.fillText('望远镜', scopeX + 12, scopeY);
  },

  onMassChange(e) {
    this.setData({ 'config.mass_kg': e.detail.value });
    this.generateReadings();
  },

  onDiameterChange(e) {
    this.setData({ 'config.diameter_mm': e.detail.value });
    this.generateReadings();
  },

  onLeverArmChange(e) {
    this.setData({ 'config.lever_arm_mm': e.detail.value });
    this.generateReadings();
  },

  onMirrorDistChange(e) {
    this.setData({ 'config.mirror_dist_m': e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const index = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({
      [`readings[${index}].reading`]: e.detail.value
    });
  },

  // 提交读数
  async submitReadings() {
    const { levelId, readings, config } = this.data;
    const payload = readings.map(r => ({
      load: r.load,
      reading: parseFloat(r.reading) || 0
    }));

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 4,
          experiment: 'young_modulus',
          readings: payload,
          user_config: {
            force_n: config.mass_kg * 9.81,
            length_m: config.length_m,
            diameter_m: config.diameter_mm * 1e-3,
            lever_arm_m: config.lever_arm_mm * 1e-3,
            mirror_dist_m: config.mirror_dist_m
          }
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
      console.error('[young-modulus] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
