// pages/experiments/lens-focal/lens-focal.js
import { request } from '../../../utils/request';

const F_TRUE = 150.0; // mm

Page({
  data: {
    levelId: null,
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
    q.select('#lfCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[lens-focal] canvas 获取失败'); return; }
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
    // 贝塞尔法：f=(L²-d²)/(4L)，取 L=700mm
    const L = 700.0;
    const d = Math.sqrt(L * L - 4.0 * L * F_TRUE);

    const readings = [];
    const Ls = [700, 680, 660, 720, 740];
    for (let i = 0; i < 5; i++) {
      const Li = Ls[i];
      const di = Math.sqrt(Li * Li - 4.0 * Li * F_TRUE);
      const noise = (Math.random() - 0.5) * 0.5;
      readings.push({
        set_num: i + 1,
        object_screen_dist_mm: Li.toFixed(1),
        lens_shift_mm: (di + noise).toFixed(2)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 光学平台（水平线）
    const benchY = h * 0.55;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(10, benchY); ctx.lineTo(w - 10, benchY);
    ctx.stroke();

    // 物（左侧箭头向上）
    const objX = w * 0.12;
    ctx.strokeStyle = '#f1c40f'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(objX, benchY - 40); ctx.lineTo(objX, benchY);
    ctx.moveTo(objX - 6, benchY - 34); ctx.lineTo(objX, benchY - 40); ctx.lineTo(objX + 6, benchY - 34);
    ctx.stroke();
    ctx.fillStyle = 'rgba(241,196,15,0.7)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('物', objX, benchY - 48);

    // 屏（右侧）
    const scrX = w * 0.88;
    ctx.strokeStyle = '#2ecc71'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(scrX, benchY - 40); ctx.lineTo(scrX, benchY);
    ctx.stroke();
    ctx.fillStyle = 'rgba(46,204,113,0.3)';
    ctx.fillRect(scrX - 2, benchY - 40, 4, 40);
    ctx.fillStyle = 'rgba(46,204,113,0.7)'; ctx.font = '9px sans-serif';
    ctx.fillText('屏', scrX, benchY - 48);

    // L 标注
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(objX, benchY + 15); ctx.lineTo(scrX, benchY + 15);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('L', (objX + scrX) / 2, benchY + 28);

    // 透镜（动画在两个位置间移动）
    const t = Date.now() / 2000;
    const cycle = (t % 1) * 2 * Math.PI;
    // 透镜位置1（大像）和位置2（小像）
    const f = F_TRUE;
    const L_mm = 700;
    const d_mm = Math.sqrt(L_mm * L_mm - 4 * L_mm * f);
    // 位置1距离物 u1 = (L+d)/2, 位置2 u2 = (L-d)/2
    const u1 = (L_mm + d_mm) / 2;
    const u2 = (L_mm - d_mm) / 2;
    // 映射到像素
    const scale = (scrX - objX) / L_mm;
    const lensX1 = objX + u1 * scale;
    const lensX2 = objX + u2 * scale;
    // 透镜在两位置间摆动
    const phase = (Math.sin(cycle) + 1) / 2;
    const lensX = lensX1 + (lensX2 - lensX1) * phase;

    // 光线（物->透镜->屏）
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.4)'; ctx.lineWidth = 1.5;
    // 光线1：物顶点 -> 透镜中心 -> 屏
    ctx.beginPath();
    ctx.moveTo(objX, benchY - 40);
    ctx.lineTo(lensX, benchY - 25);
    ctx.lineTo(scrX, benchY - 40);
    ctx.stroke();
    // 光线2：物顶点 -> 透镜顶部 -> 屏
    ctx.beginPath();
    ctx.moveTo(objX, benchY - 40);
    ctx.lineTo(lensX, benchY - 45);
    ctx.lineTo(scrX, benchY - 40);
    ctx.stroke();

    // 透镜（双凸透镜符号）
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.ellipse(lensX, benchY - 25, 5, 20, 0, 0, Math.PI * 2);
    ctx.stroke();
    ctx.fillStyle = 'rgba(52, 152, 219, 0.2)';
    ctx.fill();
    ctx.fillStyle = 'rgba(52,152,219,0.7)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('透镜', lensX, benchY - 52);

    // d 标注（两透镜位置之间）
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.5)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(lensX1, benchY - 60); ctx.lineTo(lensX2, benchY - 60);
    ctx.stroke();
    ctx.setLineDash([]);
    // 位置标记
    ctx.fillStyle = 'rgba(231, 76, 60, 0.6)';
    ctx.beginPath(); ctx.arc(lensX1, benchY - 60, 2, 0, Math.PI * 2); ctx.fill();
    ctx.beginPath(); ctx.arc(lensX2, benchY - 60, 2, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = 'rgba(231, 76, 60, 0.7)'; ctx.font = '9px sans-serif';
    ctx.fillText(`d=${d_mm.toFixed(1)}mm`, (lensX1 + lensX2) / 2, benchY - 68);

    // 上方：成像示意
    const imgY = h * 0.12;
    ctx.fillStyle = 'rgba(255,255,255,0.4)'; ctx.font = '10px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText(phase < 0.5 ? '放大像' : '缩小像', 16, imgY);

    // 底部公式
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`f = (L² - d²) / (4L) = ${F_TRUE.toFixed(1)} mm`, w / 2, h - 5);
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    const payload = readings.map(r => ({
      set_num: r.set_num,
      object_screen_dist_mm: parseFloat(r.object_screen_dist_mm) || 0,
      lens_shift_mm: parseFloat(r.lens_shift_mm) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 19,
          experiment: 'lens_focal',
          readings: payload,
          user_config: {}
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
      console.error('[lens-focal] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
