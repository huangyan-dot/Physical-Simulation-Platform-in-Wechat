// pages/experiments/photoelectric/photoelectric.js
import { request } from '../../../utils/request';

const h_planck = 6.626e-34;  // J·s
const e_charge = 1.602e-19;  // C
const c_light  = 3.0e8;      // m/s
const W_work   = 3.0e-19;    // J (钾逸出功)

Page({
  data: {
    levelId: null,
    config: { wavelength_nm: 405 },
    freqStr: '7.41',
    stopVStr: '1.04',
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
    const query = wx.createSelectorQuery().in(this);
    query.select('#peCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[photoelectric] canvas 节点获取失败'); return; }
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
      if (data.config && data.config.wavelengths_nm) {
        this.generateReadings(data.config.wavelengths_nm);
        return;
      }
    } catch (err) { console.warn('[photoelectric] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings(wavelengths) {
    const wls = wavelengths || [405, 436, 546, 577];
    const readings = wls.map(wl => {
      const nu = c_light / (wl * 1e-9);
      const Vs = (h_planck / e_charge) * nu - W_work / e_charge;
      return {
        wavelength_nm: wl,
        frequency_hz: nu,
        freq_str: (nu / 1e14).toFixed(2),
        stop_v: Vs.toFixed(3)
      };
    });
    this.setData({ readings });
    this.updateCurrent();
  },

  updateCurrent() {
    const wl = this.data.config.wavelength_nm;
    const nu = c_light / (wl * 1e-9);
    const Vs = (h_planck / e_charge) * nu - W_work / e_charge;
    this.setData({ freqStr: (nu / 1e14).toFixed(2), stopVStr: Vs.toFixed(2) });
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, config } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0a0a1a'; ctx.fillRect(0, 0, w, h);

    // 光电管示意图
    const tubeX = w * 0.15, tubeY = h * 0.15, tubeW = w * 0.5, tubeH = h * 0.35;
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.strokeRect(tubeX, tubeY, tubeW, tubeH);

    // 阴极 K
    ctx.fillStyle = '#e74c3c';
    ctx.fillRect(tubeX + 10, tubeY + 5, 8, tubeH - 10);
    ctx.fillStyle = 'rgba(255,255,255,0.6)';
    ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('K(阴极)', tubeX + 14, tubeY + tubeH + 15);

    // 阳极 A
    ctx.fillStyle = '#2ecc71';
    ctx.fillRect(tubeX + tubeW - 18, tubeY + 5, 8, tubeH - 10);
    ctx.fillText('A(阳极)', tubeX + tubeW - 14, tubeY + tubeH + 15);

    // 光源
    const lightX = w * 0.05;
    const lightY = tubeY + tubeH / 2;
    const wl = config.wavelength_nm;
    let lightColor;
    if (wl < 450) lightColor = '#4040ff';
    else if (wl < 500) lightColor = '#00aaff';
    else if (wl < 570) lightColor = '#00ff00';
    else if (wl < 590) lightColor = '#ffff00';
    else if (wl < 620) lightColor = '#ff8800';
    else lightColor = '#ff0000';

    ctx.fillStyle = lightColor;
    ctx.beginPath(); ctx.arc(lightX, lightY, 6, 0, Math.PI * 2); ctx.fill();
    // 光线
    ctx.strokeStyle = lightColor; ctx.lineWidth = 2;
    ctx.setLineDash([4, 3]);
    ctx.beginPath(); ctx.moveTo(lightX + 6, lightY); ctx.lineTo(tubeX, lightY); ctx.stroke();
    ctx.setLineDash([]);

    // 电子飞出动画
    const t = Date.now() / 200;
    for (let i = 0; i < 3; i++) {
      const progress = ((t + i * 0.7) % 3) / 3;
      const ex = tubeX + 18 + progress * (tubeW - 36);
      const ey = lightY + Math.sin(progress * Math.PI * 2 + i) * 15;
      if (ex < tubeX + tubeW - 20) {
        ctx.fillStyle = 'rgba(241, 196, 15, 0.7)';
        ctx.beginPath(); ctx.arc(ex, ey, 2, 0, Math.PI * 2); ctx.fill();
      }
    }

    // V-s 曲线（右下）
    const graphX = w * 0.55, graphY = h * 0.58, graphW = w * 0.4, graphH = h * 0.35;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);
    // 坐标轴
    ctx.strokeStyle = 'rgba(255,255,255,0.4)';
    ctx.beginPath(); ctx.moveTo(graphX, graphY + graphH); ctx.lineTo(graphX + graphW, graphY + graphH); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(graphX, graphY + graphH); ctx.lineTo(graphX, graphY); ctx.stroke();
    ctx.fillStyle = 'rgba(255,255,255,0.4)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText('V_s', graphX + 3, graphY + 10);
    ctx.fillText('ν', graphX + graphW - 8, graphY + graphH - 3);

    // 理论直线 V_s = (h/e)ν - W/e
    ctx.strokeStyle = lightColor; ctx.lineWidth = 2;
    ctx.beginPath();
    const nuMin = c_light / (700e-9), nuMax = c_light / (200e-9);
    const VsMin = (h_planck / e_charge) * nuMin - W_work / e_charge;
    const VsMax = (h_planck / e_charge) * nuMax - W_work / e_charge;
    const vRange = VsMax - VsMin;
    for (let px = 0; px <= graphW; px++) {
      const nu = nuMin + (nuMax - nuMin) * px / graphW;
      const Vs = (h_planck / e_charge) * nu - W_work / e_charge;
      const py = graphY + graphH - ((Vs - VsMin) / vRange) * graphH;
      if (Vs < 0) continue;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText(`λ = ${wl} nm`, 8, h - 8);
  },

  onWavelengthChange(e) {
    this.setData({ 'config.wavelength_nm': e.detail.value });
    this.updateCurrent();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({ [`readings[${idx}].stop_v`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    const payload = readings.map(r => ({
      frequency_hz: r.frequency_hz,
      stop_v: parseFloat(r.stop_v) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: { level_id: parseInt(levelId, 10) || 7, experiment: 'photoelectric', readings: payload }
      });
      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => { if (data.passed) wx.navigateBack(); }
      });
    } catch (err) {
      console.error('[photoelectric] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
