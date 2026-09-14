// pages/experiments/spectrometer/spectrometer.js
import { request } from '../../../utils/request';

const N_TRUE = 1.52; // 冕牌玻璃折射率

function calcMinDeviation(A_deg, n) {
  // δ = 2*arcsin(n*sin(A/2)) - A
  const A = A_deg * Math.PI / 180.0;
  const delta = 2.0 * Math.asin(n * Math.sin(A / 2.0)) - A;
  return delta * 180.0 / Math.PI;
}

Page({
  data: {
    levelId: null,
    prismAngle: 60,
    nTheoretical: '1.5200',
    deltaTheoretical: '38.92',
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
    q.select('#spCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[spectrometer] canvas 获取失败'); return; }
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
      if (data.config && data.config.prism_angle_deg) {
        this.setData({ prismAngle: data.config.prism_angle_deg });
      }
    } catch (err) { console.warn('[spectrometer] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const A = this.data.prismAngle;
    const delta = calcMinDeviation(A, N_TRUE);
    this.setData({
      nTheoretical: N_TRUE.toFixed(4),
      deltaTheoretical: delta.toFixed(2)
    });

    const readings = [];
    for (let i = 1; i <= 5; i++) {
      const noise = (Math.random() - 0.5) * 0.1; // ±0.05°
      readings.push({
        set_num: i,
        min_deviation_deg: (delta + noise).toFixed(2)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, prismAngle } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 上半部分：光路图（入射光 -> 棱镜 -> 出射光，最小偏向角）
    const cx = w / 2, cy = h * 0.38;
    const prismSize = Math.min(w, h) * 0.18;

    // 棱镜（等腰三角形）
    const A_rad = prismAngle * Math.PI / 180.0;
    const halfA = A_rad / 2.0;
    // 顶点：上、左下、右下
    const topX = cx, topY = cy - prismSize;
    const blX = cx - prismSize * Math.sin(halfA) / Math.cos(halfA) * 0.8;
    const blY = cy + prismSize * 0.6;
    const brX = cx + prismSize * Math.sin(halfA) / Math.cos(halfA) * 0.8;
    const brY = cy + prismSize * 0.6;

    // 绘制棱镜
    ctx.fillStyle = 'rgba(100, 180, 255, 0.15)';
    ctx.strokeStyle = 'rgba(100, 180, 255, 0.5)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(topX, topY);
    ctx.lineTo(blX, blY);
    ctx.lineTo(brX, brY);
    ctx.closePath();
    ctx.fill();
    ctx.stroke();

    // 入射光（从左侧水平射入棱镜左面）
    const inY = cy - prismSize * 0.2;
    ctx.strokeStyle = '#f1c40f'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(0, inY);
    // 折射进入棱镜
    const refractInX = blX + (topX - blX) * 0.5;
    const refractInY = blY + (topY - blY) * 0.5;
    ctx.lineTo(refractInX, refractInY);
    ctx.stroke();

    // 棱镜内部光线
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.5)'; ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(refractInX, refractInY);
    const refractOutX = brX + (topX - brX) * 0.5;
    const refractOutY = brY + (topY - brY) * 0.5;
    ctx.lineTo(refractOutX, refractOutY);
    ctx.stroke();

    // 出射光
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(refractOutX, refractOutY);
    const outY = refractOutY - prismSize * 0.3;
    ctx.lineTo(w, outY);
    ctx.stroke();

    // 顶角标注
    ctx.fillStyle = 'rgba(255,255,255,0.6)'; ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`A=${prismAngle}°`, topX, topY - 8);

    // 偏向角标注
    ctx.fillStyle = 'rgba(231, 76, 60, 0.7)';
    ctx.fillText('δ', w - 30, (inY + outY) / 2);

    // 下半部分：n-A 关系曲线
    const graphX = w * 0.1, graphY = h * 0.6, graphW = w * 0.8, graphH = h * 0.32;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);

    // 绘制 δ(A) 曲线
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const A_deg = 45 + (px / graphW) * 25; // 45°-70°
      const delta = calcMinDeviation(A_deg, N_TRUE);
      // δ 范围大约 30°-50°，映射到 graphH
      const py = graphY + graphH - ((delta - 25) / 30) * graphH;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 当前点
    const curDelta = calcMinDeviation(prismAngle, N_TRUE);
    const curPx = ((prismAngle - 45) / 25) * graphW;
    const curPy = graphY + graphH - ((curDelta - 25) / 30) * graphH;
    ctx.fillStyle = '#e74c3c';
    ctx.beginPath(); ctx.arc(graphX + curPx, curPy, 4, 0, Math.PI * 2); ctx.fill();

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText('A (°)', graphX + graphW - 30, graphY + graphH + 12);
    ctx.fillText('δ (°)', graphX - 5, graphY - 3);
    ctx.textAlign = 'center';
    ctx.fillText(`n=${N_TRUE.toFixed(2)}, δ=${curDelta.toFixed(1)}°`, w / 2, h - 5);
  },

  onAngleChange(e) {
    this.setData({ prismAngle: e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({ [`readings[${idx}].min_deviation_deg`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, prismAngle } = this.data;
    const payload = readings.map(r => ({
      set_num: r.set_num,
      min_deviation_deg: parseFloat(r.min_deviation_deg) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 14,
          experiment: 'spectrometer',
          readings: payload,
          user_config: { prism_angle_deg: prismAngle }
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
      console.error('[spectrometer] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
