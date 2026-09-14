// pages/experiments/frank-hertz/frank-hertz.js
import { request } from '../../../utils/request';

const EXC_V = 4.9; // 汞原子激发电位

Page({
  data: {
    levelId: null,
    vg2: 15,
    currentStr: '0.00',
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
    query.select('#fhCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[frank-hertz] canvas 节点获取失败'); return; }
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
    } catch (err) { console.warn('[frank-hertz] 后端不可用'); }
    this.generateReadings();
  },

  // 生成 I-V 数据（理论曲线，激发电位 4.9V）
  calcCurrent(V) {
    const cycle = Math.floor(V / EXC_V);
    const phase = V - cycle * EXC_V;
    let I;
    if (phase < 4.0) {
      I = 0.2 * (cycle * 0.5 + phase / 4.0);
    } else {
      const drop = (phase - 4.0) / 0.9;
      I = 0.2 * (cycle * 0.5 + 1.0) * (1 - drop * 0.8);
    }
    return Math.max(0, I);
  },

  generateReadings() {
    // 采样 0~30V，每 1V 记一个点
    const readings = [];
    for (let V = 1; V <= 25; V += 1) {
      readings.push({ voltage: V.toFixed(1), current: this.calcCurrent(V).toFixed(3) });
    }
    this.setData({ readings });
    this.updateCurrent();
  },

  updateCurrent() {
    const I = this.calcCurrent(this.data.vg2);
    this.setData({ currentStr: I.toFixed(2) });
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, vg2 } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 绘制 I-V 曲线图
    const padL = 40, padR = 15, padT = 20, padB = 30;
    const graphW = w - padL - padR, graphH = h - padT - padB;
    const Vmax = 30, Imax = 2.0;

    // 网格
    ctx.strokeStyle = 'rgba(255,255,255,0.08)'; ctx.lineWidth = 1;
    for (let i = 0; i <= 6; i++) {
      const x = padL + graphW * i / 6;
      ctx.beginPath(); ctx.moveTo(x, padT); ctx.lineTo(x, padT + graphH); ctx.stroke();
    }
    for (let i = 0; i <= 4; i++) {
      const y = padT + graphH * i / 4;
      ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(padL + graphW, y); ctx.stroke();
    }

    // 坐标轴
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 1.5;
    ctx.beginPath(); ctx.moveTo(padL, padT); ctx.lineTo(padL, padT + graphH); ctx.lineTo(padL + graphW, padT + graphH); ctx.stroke();

    // 刻度标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    for (let i = 0; i <= 6; i++) {
      const V = i * 5;
      ctx.fillText(V + 'V', padL + graphW * i / 6, padT + graphH + 14);
    }
    ctx.textAlign = 'right';
    for (let i = 0; i <= 4; i++) {
      const I = (4 - i) * 0.5;
      ctx.fillText(I.toFixed(1), padL - 4, padT + graphH * i / 4 + 3);
    }
    ctx.textAlign = 'left';
    ctx.fillText('I(μA)', padL - 25, padT - 5);
    ctx.fillText('V_G2(V)', padL + graphW - 30, padT + graphH + 24);

    // 绘制 I-V 理论曲线
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    let started = false;
    for (let px = 0; px <= graphW; px++) {
      const V = (px / graphW) * Vmax;
      const I = this.calcCurrent(V);
      const py = padT + graphH - (I / Imax) * graphH;
      if (py < padT) continue;
      if (!started) { ctx.moveTo(padL + px, py); started = true; }
      else ctx.lineTo(padL + px, py);
    }
    ctx.stroke();

    // 标记峰位
    const peaks = [4.9, 9.8, 14.7, 19.6, 24.5];
    ctx.fillStyle = '#e74c3c';
    for (const pk of peaks) {
      if (pk > Vmax) continue;
      const px = padL + (pk / Vmax) * graphW;
      const I = this.calcCurrent(pk - 0.3); // 峰位略前
      const py = padT + graphH - (I / Imax) * graphH;
      ctx.beginPath(); ctx.arc(px, py, 3, 0, Math.PI * 2); ctx.fill();
    }

    // 当前电压指示线
    const curX = padL + (vg2 / Vmax) * graphW;
    const curI = this.calcCurrent(vg2);
    const curY = padT + graphH - (curI / Imax) * graphH;
    ctx.strokeStyle = '#f1c40f'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath(); ctx.moveTo(curX, padT + graphH); ctx.lineTo(curX, curY); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(padL, curY); ctx.lineTo(curX, curY); ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = '#f1c40f';
    ctx.beginPath(); ctx.arc(curX, curY, 4, 0, Math.PI * 2); ctx.fill();

    // 弗兰克-赫兹管示意图（左上角小图）
    const miniX = padL + 5, miniY = padT + 5, miniW = 60, miniH = 25;
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 1;
    ctx.strokeRect(miniX, miniY, miniW, miniH);
    // 阴极
    ctx.fillStyle = '#e74c3c'; ctx.fillRect(miniX + 3, miniY + 3, 3, miniH - 6);
    // 棚极
    ctx.fillStyle = '#f1c40f'; ctx.fillRect(miniX + miniW / 2 - 1, miniY + 3, 2, miniH - 6);
    // 阳极
    ctx.fillStyle = '#2ecc71'; ctx.fillRect(miniX + miniW - 6, miniY + 3, 3, miniH - 6);
    ctx.fillStyle = 'rgba(255,255,255,0.4)'; ctx.font = '7px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('K', miniX + 5, miniY + miniH + 8);
    ctx.fillText('G', miniX + miniW / 2, miniY + miniH + 8);
    ctx.fillText('A', miniX + miniW - 5, miniY + miniH + 8);
  },

  onVg2Change(e) {
    this.setData({ vg2: e.detail.value });
    this.updateCurrent();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({ [`readings[${idx}].current`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    const payload = readings.map(r => ({
      voltage_v: parseFloat(r.voltage) || 0,
      current_a: (parseFloat(r.current) || 0) * 1e-6 // μA -> A
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: { level_id: parseInt(levelId, 10) || 8, experiment: 'frank_hertz', readings: payload }
      });
      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => { if (data.passed) wx.navigateBack(); }
      });
    } catch (err) {
      console.error('[frank-hertz] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
