// pages/experiments/newton-ring/newton-ring.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      wavelength_nm: 589.3,
      lens_radius_mm: 855,
      k_range: [1, 10],
      tolerance_mm: 0.02
    },
    drumReading: 0,
    readings: [],
    submitting: false,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  // 缓存的 canvas 节点和上下文
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
    // 缓存 canvas 节点，后续直接用缓存重绘
    const query = wx.createSelectorQuery().in(this);
    query.select('#ringCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[newton-ring] canvas 节点获取失败');
          return;
        }
        const canvas = res[0].node;
        const ctx = canvas.getContext('2d');
        this._canvas = canvas;
        this._ctx = ctx;

        // 启动持续重绘循环（与单摆一致，确保 setData 后自动刷新画面）
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

  // 初始化 canvas 尺寸
  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    const height = Math.min(width, 480);
    const pr = sysInfo.pixelRatio || 1;
    this.setData({
      canvasWidth: width,
      canvasHeight: height,
      pixelRatio: pr
    });
  },

  // 加载实验配置
  async loadExperimentConfig(levelId) {
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      const data = res.data || res || {};
      const config = { ...this.data.config, ...(data.config || {}) };
      this.setData({ experimentId: data.id, config });
    } catch (err) {
      console.warn('[newton-ring] 后端不可用，使用默认配置');
    }
    this.generateReadings();
    // rAF 循环会自动拾取新 config 绘制
  },

  // 生成读数表
  generateReadings() {
    const { config } = this.data;
    const [kMin, kMax] = config.k_range;
    const readings = [];
    for (let k = kMin; k <= kMax; k++) {
      const theoretical = this.calculateRadius(k);
      readings.push({
        k,
        theoretical,
        measured: theoretical.toFixed(3)
      });
    }
    this.setData({ readings });
  },

  // 计算第 k 级暗环半径（单位：mm）
  calculateRadius(k) {
    const { wavelength_nm, lens_radius_mm } = this.data.config;
    const wavelength_mm = wavelength_nm * 1e-6; // nm -> mm
    return Math.sqrt(k * wavelength_mm * lens_radius_mm);
  },

  // 持续重绘循环（rAF 自动拾取最新 this.data，与单摆同模式）
  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => {
      this.drawRings();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  // 绘制牛顿环（使用缓存的 canvas 节点）
  drawRings() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) {
      console.warn('[newton-ring] canvas 未就绪，延迟绘制');
      return;
    }

    const { canvasWidth, canvasHeight, pixelRatio, config, drumReading } = this.data;

    // 设置 canvas 像素尺寸
    canvas.width = canvasWidth * pixelRatio;
    canvas.height = canvasHeight * pixelRatio;

    // 重置变换矩阵（避免 scale 累积）
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    const cx = canvasWidth / 2;
    const cy = canvasHeight / 2;
    const maxRadius = Math.min(cx, cy) * 0.9;

    // 清空画布
    ctx.fillStyle = '#0a0a0a';
    ctx.fillRect(0, 0, canvasWidth, canvasHeight);

    // 绘制光斑背景
    const gradient = ctx.createRadialGradient(cx, cy, 0, cx, cy, maxRadius);
    gradient.addColorStop(0, '#1a1a2e');
    gradient.addColorStop(1, '#000000');
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(cx, cy, maxRadius, 0, Math.PI * 2);
    ctx.fill();

    // 绘制同心暗环
    const [kMin, kMax] = config.k_range;
    const kLimit = Math.max(kMax, 20);

    // 固定参考比例尺：以参数范围中值计算，避免 λ/R 在归一化时被约掉
    // 波长范围 400-700nm → 参考 550nm；曲率半径 500-1200mm → 参考 850mm
    const refWavelength_mm = 550 * 1e-6;
    const refLensR = 850;
    const refMaxRadiusMM = Math.sqrt(kLimit * refWavelength_mm * refLensR);
    const scale = maxRadius / refMaxRadiusMM; // 像素/mm，固定不变

    ctx.lineWidth = 1.5;
    for (let k = kMin; k <= kLimit; k++) {
      const r_mm = this.calculateRadius(k);
      const r_px = r_mm * scale + drumReading * 5;

      if (r_px > maxRadius) break;

      const alpha = Math.max(0.15, 0.8 - k * 0.03);
      ctx.strokeStyle = `rgba(255, 255, 255, ${alpha})`;
      ctx.beginPath();
      ctx.arc(cx, cy, r_px, 0, Math.PI * 2);
      ctx.stroke();
    }

    // 绘制中心点
    ctx.fillStyle = '#fff';
    ctx.beginPath();
    ctx.arc(cx, cy, 2, 0, Math.PI * 2);
    ctx.fill();
  },

  // 参数变化
  onWavelengthChange(e) {
    this.setData({ 'config.wavelength_nm': e.detail.value });
    this.generateReadings();
  },

  onRadiusChange(e) {
    this.setData({ 'config.lens_radius_mm': e.detail.value });
    this.generateReadings();
  },

  onDrumChange(e) {
    this.setData({ drumReading: e.detail.value });
  },

  // 读数输入
  onReadingInput(e) {
    const index = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({
      [`readings[${index}].measured`]: e.detail.value
    });
  },

  onCanvasTouch() {
    // 预留：可拖动十字准线测量
  },

  // 提交读数
  async submitReadings() {
    const { levelId, readings, config } = this.data;
    const payload = readings.map(r => ({
      k: r.k,
      r: parseFloat(r.measured) || 0
    }));

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 1,
          experiment: 'newton_ring',
          readings: payload,
          // 携带前端实际实验参数，后端据此动态生成 target（滑块问题修复）
          user_config: {
            lens_radius_mm: config.lens_radius_mm,
            wavelength_nm: config.wavelength_nm
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
      console.error('[newton-ring] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});