// pages/experiments/sound-speed/sound-speed.js
import { request } from '../../../utils/request';

const SPEED = 343.0; // m/s

Page({
  data: {
    levelId: null,
    freqKHz: 37,
    lambdaStr: '9.27',
    readings: [],
    submitting: false,
    canvasWidth: 0, canvasHeight: 0, pixelRatio: 1
  },
  _canvas: null, _ctx: null, _rafId: 0,

  onLoad(options) {
    this.setData({ levelId: options.levelId });
    this.initCanvas();
    if (options.levelId) {
      this.loadExperimentConfig(options.levelId);
    } else {
      this.generateReadings();
    }
  },

  onReady() {
    const q = wx.createSelectorQuery().in(this);
    q.select('#ssCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[sound-speed] canvas 获取失败'); return; }
      this._canvas = res[0].node;
      this._ctx = this._canvas.getContext('2d');
      this.startAnimation();
    });
  },

  onUnload() {
    if (this._rafId && this._canvas) this._canvas.cancelAnimationFrame(this._rafId);
    this._canvas = null; this._ctx = null;
  },

  initCanvas() {
    const sys = wx.getSystemInfoSync();
    const w = sys.windowWidth;
    this.setData({ canvasWidth: w, canvasHeight: Math.min(w, 420), pixelRatio: sys.pixelRatio || 1 });
  },

  async loadExperimentConfig(levelId) {
    try {
      const res = await request({ url: `/experiments/${levelId}`, method: 'GET' });
      const data = res.data || res || {};
      if (data.config && data.config.freq_hz) {
        this.setData({ freqKHz: Math.round(data.config.freq_hz / 1000) });
      }
    } catch (err) { console.warn('[sound-speed] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const freq = this.data.freqKHz * 1000;
    const lambda = SPEED / freq; // m
    const halfLambdaMM = (lambda / 2) * 1000; // mm
    this.setData({ lambdaStr: (lambda * 1000).toFixed(2) });

    const readings = [];
    for (let i = 1; i <= 8; i++) {
      readings.push({ order: i, position_mm: (i * halfLambdaMM).toFixed(2) });
    }
    this.setData({ readings });
  },

  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => { this.drawScene(); this._rafId = canvas.requestAnimationFrame(loop); };
    loop();
  },

  drawScene() {
    const canvas = this._canvas, ctx = this._ctx;
    if (!canvas || !ctx) return;
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, freqKHz } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    const cy = h * 0.35;

    // 超声波发射器（左）
    const txX = w * 0.15;
    ctx.fillStyle = '#3498db';
    ctx.fillRect(txX - 15, cy - 20, 15, 40);
    ctx.fillStyle = 'rgba(255,255,255,0.5)';
    ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('发射器', txX - 8, cy + 35);

    // 超声波接收器（右，可移动）
    const rxX = w * 0.7;
    ctx.fillStyle = '#e74c3c';
    ctx.fillRect(rxX, cy - 20, 15, 40);
    ctx.fillText('接收器', rxX + 8, cy + 35);

    // 声波（驻波模式）
    const t = Date.now() / 100;
    const freq = freqKHz * 1000;
    const lambda = SPEED / freq;
    const wavelengthPx = 60; // 显示用固定像素波长
    const amplitude = 15;

    ctx.strokeStyle = 'rgba(52, 152, 219, 0.6)'; ctx.lineWidth = 1.5;
    ctx.beginPath();
    for (let px = txX; px <= rxX; px += 1) {
      const phase = (px - txX) / wavelengthPx * Math.PI * 2;
      // 驻波：sin(kx)·cos(ωt)
      const envelope = Math.abs(Math.sin(phase));
      const wave = envelope * Math.cos(t) * amplitude;
      if (px === txX) ctx.moveTo(px, cy + wave);
      else ctx.lineTo(px, cy + wave);
    }
    ctx.stroke();

    // 驻波包络线
    ctx.strokeStyle = 'rgba(52, 152, 219, 0.2)'; ctx.lineWidth = 1;
    ctx.beginPath();
    for (let px = txX; px <= rxX; px += 1) {
      const phase = (px - txX) / wavelengthPx * Math.PI * 2;
      const env = Math.abs(Math.sin(phase)) * amplitude;
      if (px === txX) ctx.moveTo(px, cy + env);
      else ctx.lineTo(px, cy + env);
    }
    ctx.stroke();
    ctx.beginPath();
    for (let px = txX; px <= rxX; px += 1) {
      const phase = (px - txX) / wavelengthPx * Math.PI * 2;
      const env = -Math.abs(Math.sin(phase)) * amplitude;
      if (px === txX) ctx.moveTo(px, cy + env);
      else ctx.lineTo(px, cy + env);
    }
    ctx.stroke();

    // 标注 λ/2
    const halfWavePx = wavelengthPx / 2;
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.5)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(txX + halfWavePx, cy - amplitude - 10);
    ctx.lineTo(txX + halfWavePx, cy + amplitude + 10);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(241, 196, 15, 0.7)';
    ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('λ/2', txX + halfWavePx, cy - amplitude - 15);

    // I-X 曲线（下方）
    const graphX = w * 0.1, graphY = h * 0.6, graphW = w * 0.8, graphH = h * 0.32;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);
    ctx.strokeStyle = 'rgba(255,255,255,0.3)';
    ctx.beginPath(); ctx.moveTo(graphX, graphY + graphH); ctx.lineTo(graphX + graphW, graphY + graphH); ctx.stroke();

    // 共振峰
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.beginPath();
    const numPeaks = 6;
    for (let px = 0; px <= graphW; px++) {
      const peakIdx = px / (graphW / numPeaks);
      const dist = Math.abs(peakIdx - Math.round(peakIdx));
      const I = Math.exp(-dist * dist * 10);
      const py = graphY + graphH - I * graphH * 0.8;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText('接收器位置', graphX, graphY + graphH + 12);
    ctx.fillText('信号强度', graphX - 5, graphY - 3);
    ctx.textAlign = 'center';
    ctx.fillText(`f = ${freqKHz} kHz`, w / 2, h - 5);
  },

  onFreqChange(e) {
    this.setData({ freqKHz: e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({ [`readings[${idx}].position_mm`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, freqKHz } = this.data;
    const payload = readings.map(r => ({
      order: r.order,
      position_m: (parseFloat(r.position_mm) || 0) * 1e-3
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 12,
          experiment: 'sound_speed',
          readings: payload,
          user_config: { freq_hz: freqKHz * 1000 }
        }
      });
      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => { if (data.passed) wx.navigateBack(); }
      });
    } catch (err) {
      console.error('[sound-speed] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
