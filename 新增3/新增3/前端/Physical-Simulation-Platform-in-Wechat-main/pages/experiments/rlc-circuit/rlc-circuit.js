// pages/experiments/rlc-circuit/rlc-circuit.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    inductanceMH: 10,      // mH
    capacitanceNF: 100,    // nF
    f0Display: '5033',
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
    q.select('#rlcCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[rlc-circuit] canvas 获取失败'); return; }
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
      if (data.config) {
        if (data.config.inductance_h) {
          this.setData({ inductanceMH: Math.round(data.config.inductance_h * 1000) });
        }
        if (data.config.capacitance_f) {
          this.setData({ capacitanceNF: Math.round(data.config.capacitance_f * 1e9) });
        }
      }
    } catch (err) { console.warn('[rlc-circuit] 后端不可用'); }
    this.generateReadings();
  },

  getF0() {
    const L = this.data.inductanceMH * 1e-3;  // H
    const C = this.data.capacitanceNF * 1e-9;  // F
    return 1.0 / (2.0 * Math.PI * Math.sqrt(L * C));
  },

  generateReadings() {
    const f0 = this.getF0();
    this.setData({ f0Display: f0.toFixed(0) });

    // 生成以 f0 为中心的 11 组数据，步进约 f0*2%
    const step = f0 * 0.02;
    const readings = [];
    for (let i = -5; i <= 5; i++) {
      const f = f0 + i * step;
      // 简化谐振曲线：高斯峰
      const I = 10.0 * Math.exp(-Math.pow((f - f0) / (step * 2.5), 2)); // mA
      const noise = (Math.random() - 0.5) * 0.1;
      readings.push({
        set_num: i + 6,
        frequency_hz: f.toFixed(1),
        current_ma: (I + noise).toFixed(3)
      });
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, inductanceMH, capacitanceNF } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    const f0 = this.getF0();

    // 上半部分：RLC 电路图
    const cy = h * 0.18;
    const startX = 30, endX = w - 30;

    // 电源（交流）
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(startX, cy); ctx.lineTo(startX, cy + 30);
    ctx.lineTo(w * 0.2, cy + 30);
    ctx.stroke();
    ctx.fillStyle = '#f1c40f'; ctx.font = 'bold 12px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('~V', startX, cy - 5);

    // R（电阻 - 矩形）
    const rX = w * 0.22;
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.strokeRect(rX, cy + 22, 30, 16);
    ctx.fillStyle = 'rgba(231,76,60,0.7)'; ctx.font = '9px sans-serif';
    ctx.fillText('R', rX + 15, cy + 18);

    // 连线
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(rX + 30, cy + 30); ctx.lineTo(w * 0.45, cy + 30);
    ctx.stroke();

    // L（电感 - 线圈）
    const lX = w * 0.45;
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    for (let i = 0; i < 4; i++) {
      ctx.beginPath();
      ctx.arc(lX + 5 + i * 8, cy + 30, 4, Math.PI, 0, false);
      ctx.stroke();
    }
    ctx.fillStyle = 'rgba(52,152,219,0.7)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('L', lX + 16, cy + 18);

    // 连线
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(lX + 37, cy + 30); ctx.lineTo(w * 0.68, cy + 30);
    ctx.stroke();

    // C（电容 - 两平行线）
    const cX = w * 0.68;
    ctx.strokeStyle = '#2ecc71'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(cX, cy + 22); ctx.lineTo(cX + 20, cy + 22);
    ctx.moveTo(cX, cy + 38); ctx.lineTo(cX + 20, cy + 38);
    ctx.stroke();
    ctx.fillStyle = 'rgba(46,204,113,0.7)'; ctx.font = '9px sans-serif';
    ctx.fillText('C', cX + 10, cy + 18);

    // 连线回到电源
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(cX + 20, cy + 30); ctx.lineTo(endX, cy + 30);
    ctx.lineTo(endX, cy);
    ctx.moveTo(endX, cy); ctx.lineTo(endX, cy);
    ctx.stroke();

    // 电流方向（闪烁箭头）
    const t = Date.now() / 400;
    const alpha = (Math.sin(t) + 1) / 2 * 0.5 + 0.3;
    ctx.fillStyle = `rgba(231, 76, 60, ${alpha})`;
    ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('→ I', w / 2, cy + 50);

    // 下半部分：I-f 谐振曲线
    const graphX = w * 0.08, graphY = h * 0.42, graphW = w * 0.84, graphH = h * 0.48;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);

    // 网格线
    ctx.strokeStyle = 'rgba(255,255,255,0.08)'; ctx.lineWidth = 0.5;
    for (let i = 1; i < 5; i++) {
      const gy = graphY + (graphH / 5) * i;
      ctx.beginPath(); ctx.moveTo(graphX, gy); ctx.lineTo(graphX + graphW, gy); ctx.stroke();
    }

    // 谐振曲线
    const step = f0 * 0.02;
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const freqOffset = (px / graphW - 0.5) * step * 12;
      const f = f0 + freqOffset;
      const I = 10.0 * Math.exp(-Math.pow((f - f0) / (step * 2.5), 2));
      const py = graphY + graphH - (I / 10) * graphH * 0.9;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 谐振峰标注
    const peakX = graphX + graphW / 2;
    const peakY = graphY + graphH - 0.9 * graphH;
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.5)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(peakX, peakY); ctx.lineTo(peakX, graphY + graphH);
    ctx.stroke();
    ctx.setLineDash([]);

    ctx.fillStyle = '#e74c3c';
    ctx.beginPath(); ctx.arc(peakX, peakY, 4, 0, Math.PI * 2); ctx.fill();

    ctx.fillStyle = 'rgba(241, 196, 15, 0.8)'; ctx.font = 'bold 10px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`f₀ = ${f0.toFixed(0)} Hz`, peakX, peakY - 10);

    // 数据点
    ctx.fillStyle = '#f1c40f';
    if (this.data.readings.length > 0) {
      for (let i = 0; i < this.data.readings.length; i++) {
        const r = this.data.readings[i];
        const f = parseFloat(r.frequency_hz);
        const I = parseFloat(r.current_ma);
        const px = graphX + ((f - f0 + step * 6) / (step * 12)) * graphW;
        const py = graphY + graphH - (I / 10) * graphH * 0.9;
        if (px >= graphX && px <= graphX + graphW) {
          ctx.beginPath(); ctx.arc(px, py, 2.5, 0, Math.PI * 2); ctx.fill();
        }
      }
    }

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('I-f 谐振曲线', graphX + graphW / 2, graphY - 5);
    ctx.textAlign = 'left';
    ctx.fillText('f (Hz)', graphX + graphW - 30, graphY + graphH + 12);
    ctx.fillText('I (mA)', graphX - 5, graphY - 3);

    // 底部参数
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`L=${inductanceMH}mH, C=${capacitanceNF}nF, f₀=${f0.toFixed(0)}Hz`, w / 2, h - 5);
  },

  onInductanceChange(e) {
    this.setData({ inductanceMH: e.detail.value });
    this.generateReadings();
  },

  onCapacitanceChange(e) {
    this.setData({ capacitanceNF: e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, inductanceMH, capacitanceNF } = this.data;
    const L = inductanceMH * 1e-3;
    const C = capacitanceNF * 1e-9;
    const payload = readings.map(r => ({
      set_num: r.set_num,
      frequency_hz: parseFloat(r.frequency_hz) || 0,
      current_ma: parseFloat(r.current_ma) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 18,
          experiment: 'rlc_circuit',
          readings: payload,
          user_config: { inductance_h: L, capacitance_f: C }
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
      console.error('[rlc-circuit] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
