// pages/experiments/polarization/polarization.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    analyzerAngle: 30,
    theoryIntensityStr: '0.750',
    measuredIntensityStr: '0.750',
    readings: [],
    submitting: false,
    canvasWidth: 0, canvasHeight: 0, pixelRatio: 1
  },
  _canvas: null, _ctx: null, _rafId: 0,

  onLoad(options) {
    this.setData({ levelId: options.levelId });
    this.initCanvas();
    this.generateReadings();
  },

  onReady() {
    const q = wx.createSelectorQuery().in(this);
    q.select('#polCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[polarization] canvas 获取失败'); return; }
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

  generateReadings() {
    const readings = [];
    for (let deg = 0; deg <= 90; deg += 10) {
      const theta = deg * Math.PI / 180;
      const I = Math.cos(theta) * Math.cos(theta);
      readings.push({ angle: deg, theory: I.toFixed(3), measured: I.toFixed(3) });
    }
    this.setData({ readings });
    this.updateCurrent();
  },

  updateCurrent() {
    const theta = this.data.analyzerAngle * Math.PI / 180;
    const I = Math.cos(theta) * Math.cos(theta);
    this.setData({ theoryIntensityStr: I.toFixed(3), measuredIntensityStr: I.toFixed(3) });
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, analyzerAngle } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0a0a1a'; ctx.fillRect(0, 0, w, h);

    const cy = h * 0.3;

    // 光源
    ctx.fillStyle = '#ffff00';
    ctx.beginPath(); ctx.arc(w * 0.08, cy, 6, 0, Math.PI * 2); ctx.fill();
    // 自然光（多方向振动）
    ctx.strokeStyle = 'rgba(255,255,0,0.5)'; ctx.lineWidth = 1;
    for (let i = 0; i < 4; i++) {
      const a = i * Math.PI / 4;
      ctx.beginPath();
      ctx.moveTo(w * 0.08 + 6, cy);
      ctx.lineTo(w * 0.08 + 6 + 15 * Math.cos(a), cy + 15 * Math.sin(a));
      ctx.stroke();
    }

    // 起偏器 P1
    const p1X = w * 0.28;
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 3;
    ctx.beginPath(); ctx.moveTo(p1X, cy - 25); ctx.lineTo(p1X, cy + 25); ctx.stroke();
    // 起偏器透振方向（竖直）
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.moveTo(p1X - 3, cy - 12); ctx.lineTo(p1X + 3, cy + 12); ctx.stroke();

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('P1', p1X, cy + 38);

    // 线偏振光（竖直振动）
    ctx.strokeStyle = '#f1c40f'; ctx.lineWidth = 1.5;
    const t1 = Date.now() / 200;
    for (let i = 0; i < 5; i++) {
      const px = p1X + 10 + i * 12;
      const py = cy + Math.sin(t1 + i) * 8;
      ctx.beginPath(); ctx.moveTo(px, py - 4); ctx.lineTo(px, py + 4); ctx.stroke();
    }

    // 检偏器 P2
    const p2X = w * 0.62;
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 3;
    ctx.beginPath(); ctx.moveTo(p2X, cy - 25); ctx.lineTo(p2X, cy + 25); ctx.stroke();
    // 检偏器透振方向（随角度旋转）
    const aRad = analyzerAngle * Math.PI / 180;
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.save();
    ctx.translate(p2X, cy);
    ctx.rotate(aRad);
    ctx.beginPath(); ctx.moveTo(-3, -12); ctx.lineTo(3, 12); ctx.stroke();
    ctx.restore();
    ctx.fillText('P2', p2X, cy + 38);

    // 透射光（强度随 cos²θ 变化）
    const cos2 = Math.cos(aRad) * Math.cos(aRad);
    const beamAlpha = 0.3 + cos2 * 0.5;
    ctx.strokeStyle = `rgba(241, 196, 15, ${beamAlpha})`; ctx.lineWidth = 2 + cos2 * 3;
    ctx.beginPath(); ctx.moveTo(p2X + 5, cy); ctx.lineTo(w * 0.92, cy); ctx.stroke();

    // 光强指示
    const indicatorX = w * 0.92;
    const indicatorR = 4 + cos2 * 12;
    ctx.fillStyle = `rgba(241, 196, 15, ${0.3 + cos2 * 0.7})`;
    ctx.beginPath(); ctx.arc(indicatorX, cy, indicatorR, 0, Math.PI * 2); ctx.fill();

    // I-θ 曲线（下方）
    const graphX = w * 0.1, graphY = h * 0.55, graphW = w * 0.8, graphH = h * 0.35;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);
    // 坐标轴
    ctx.strokeStyle = 'rgba(255,255,255,0.4)';
    ctx.beginPath(); ctx.moveTo(graphX, graphY + graphH); ctx.lineTo(graphX + graphW, graphY + graphH); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(graphX, graphY + graphH); ctx.lineTo(graphX, graphY); ctx.stroke();

    // cos²θ 曲线
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const deg = (px / graphW) * 90;
      const I = Math.cos(deg * Math.PI / 180) ** 2;
      const py = graphY + graphH - I * graphH;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 当前角度指示
    const curPx = graphX + (analyzerAngle / 90) * graphW;
    const curI = Math.cos(analyzerAngle * Math.PI / 180) ** 2;
    const curPy = graphY + graphH - curI * graphH;
    ctx.strokeStyle = '#f1c40f'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath(); ctx.moveTo(curPx, graphY + graphH); ctx.lineTo(curPx, curPy); ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = '#f1c40f';
    ctx.beginPath(); ctx.arc(curPx, curPy, 4, 0, Math.PI * 2); ctx.fill();

    // 标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('θ', graphX + graphW - 5, graphY + graphH + 12);
    ctx.fillText('I', graphX - 8, graphY + 8);
    ctx.fillText(analyzerAngle + '°', curPx, graphY + graphH + 12);
  },

  onAngleChange(e) {
    this.setData({ analyzerAngle: e.detail.value });
    this.updateCurrent();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({ [`readings[${idx}].measured`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    const payload = readings.map(r => ({
      angle_deg: parseFloat(r.angle) || 0,
      intensity: parseFloat(r.measured) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: { level_id: parseInt(levelId, 10) || 11, experiment: 'polarization', readings: payload }
      });
      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => { if (data.passed) wx.navigateBack(); }
      });
    } catch (err) {
      console.error('[polarization] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
