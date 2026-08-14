// pages/experiments/oscilloscope/oscilloscope.js
import { request } from '../../../utils/request';
import { getTheory } from '../../../utils/theory';
import { pickQuiz, checkAnswer, QUIZ_SIZE } from '../../../utils/quiz';

// 档位（真实示波器的 1-2-5 序列）
const VOLT_DIVS = [0.1, 0.2, 0.5, 1, 2, 5];        // V/div
const TIME_DIVS = [0.1, 0.2, 0.5, 1, 2, 5, 10];    // ms/div
const GRID_X = 10; // 水平 10 格
const GRID_Y = 8;  // 竖直 8 格

Page({
  data: {
    tab: 'theory',
    levelId: null,
    assignmentId: null,
    theory: null,

    mode: 'CH1',
    modes: ['CH1', 'CH2', '李萨如'],
    // 真实信号（学生不可见，需自己数格子测出）
    ch1: { A: 2.0, f: 50, phi: 0 },
    ch2: { A: 1.5, f: 50, phi: Math.PI / 4 },

    // 档位
    voltDivs: VOLT_DIVS,
    timeDivs: TIME_DIVS,
    voltIdx: 3,   // 1 V/div
    timeIdx: 3,   // 1 ms/div
    voltDiv: 1,
    timeDiv: 1,

    // 学生数格子填入的结果
    readings: {
      ch1_vdiv: '', ch1_tdiv: '',   // 峰峰占几格、一周期占几格
      ch2_vdiv: '', ch2_tdiv: ''
    },
    // 由格数算出的物理量
    derived: { ch1_A: null, ch1_f: null, ch2_A: null, ch2_f: null },

    submitting: false,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1,

    quiz: [],
    quizAnswers: {},
    quizState: {},
    quizScore: null,
    quizSize: QUIZ_SIZE
  },

  _canvas: null,
  _ctx: null,
  _rafId: 0,

  onLoad(options) {
    const levelId = options.levelId;
    const assignmentId = options.assignmentId || null;
    this.setData({
      levelId,
      assignmentId,
      theory: getTheory('oscilloscope'),
      quiz: pickQuiz('oscilloscope'),
      voltDiv: VOLT_DIVS[this.data.voltIdx],
      timeDiv: TIME_DIVS[this.data.timeIdx]
    });
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    }
  },

  async loadExperimentConfig(levelId) {
    try {
      const data = await request({ url: `/experiments/${levelId}`, method: 'GET' });
      const channels = (data.config && data.config.channels) || {};
      const patch = {};
      // 只更新真实信号，不再预填读数（学生须自己测）
      if (channels.CH1) patch.ch1 = { ...this.data.ch1, ...channels.CH1 };
      if (channels.CH2) patch.ch2 = { ...this.data.ch2, ...channels.CH2 };
      if (Object.keys(patch).length) this.setData(patch);
    } catch (err) {
      console.error('[oscilloscope] 加载实验配置失败，使用页面默认参数:', err);
      wx.showToast({ title: '实验参数加载失败，使用默认值', icon: 'none' });
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#scopeCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[oscilloscope] canvas 节点获取失败');
          return;
        }
        this._canvas = res[0].node;
        this._ctx = this._canvas.getContext('2d');
        this.startAnimation();
      });
  },

  onUnload() {
    if (this._rafId && this._canvas) this._canvas.cancelAnimationFrame(this._rafId);
    this._canvas = null;
    this._ctx = null;
  },

  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    // 保证格子是正方形：高 = 宽 * (GRID_Y/GRID_X)
    const height = Math.round((width * GRID_Y) / GRID_X);
    this.setData({
      canvasWidth: width,
      canvasHeight: height,
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  switchTab(e) {
    this.setData({ tab: e.currentTarget.dataset.tab });
  },

  onModeChange(e) {
    this.setData({ mode: this.data.modes[e.detail.value] });
  },

  // 档位切换
  onVoltDivChange(e) {
    const i = Number(e.detail.value);
    this.setData({ voltIdx: i, voltDiv: VOLT_DIVS[i] });
  },

  onTimeDivChange(e) {
    const i = Number(e.detail.value);
    this.setData({ timeIdx: i, timeDiv: TIME_DIVS[i] });
  },

  // ===== 绘制 =====
  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => {
      this.drawScope();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  drawScope() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth: w, canvasHeight: h, pixelRatio, mode } = this.data;

    canvas.width = w * pixelRatio;
    canvas.height = h * pixelRatio;
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    // 屏幕背景
    ctx.fillStyle = '#071407';
    ctx.fillRect(0, 0, w, h);

    const gx = w / GRID_X;
    const gy = h / GRID_Y;

    // 细网格（每格 5 小格）
    ctx.strokeStyle = 'rgba(0,255,0,0.07)';
    ctx.lineWidth = 1;
    for (let i = 0; i <= GRID_X * 5; i++) {
      const x = (i * gx) / 5;
      ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, h); ctx.stroke();
    }
    for (let i = 0; i <= GRID_Y * 5; i++) {
      const y = (i * gy) / 5;
      ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(w, y); ctx.stroke();
    }

    // 主网格
    ctx.strokeStyle = 'rgba(0,255,0,0.22)';
    for (let i = 0; i <= GRID_X; i++) {
      ctx.beginPath(); ctx.moveTo(i * gx, 0); ctx.lineTo(i * gx, h); ctx.stroke();
    }
    for (let i = 0; i <= GRID_Y; i++) {
      ctx.beginPath(); ctx.moveTo(0, i * gy); ctx.lineTo(w, i * gy); ctx.stroke();
    }

    // 中心十字（刻度更密）
    ctx.strokeStyle = 'rgba(0,255,0,0.4)';
    ctx.beginPath(); ctx.moveTo(0, h / 2); ctx.lineTo(w, h / 2); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(w / 2, 0); ctx.lineTo(w / 2, h); ctx.stroke();

    if (mode === '李萨如') {
      this.drawLissajous(ctx, w, h);
    } else {
      this.drawWaveform(ctx, w, h);
    }

    // 档位标注
    ctx.fillStyle = 'rgba(0,255,0,0.75)';
    ctx.font = '12px monospace';
    ctx.textAlign = 'left';
    ctx.fillText(`${this.data.voltDiv} V/div`, 8, 16);
    ctx.textAlign = 'right';
    ctx.fillText(`${this.data.timeDiv} ms/div`, w - 8, 16);
    ctx.textAlign = 'left';
    ctx.fillText(this.data.mode, 8, h - 8);
  },

  // 时域波形：按档位换算，实现真实的"数格子"
  drawWaveform(ctx, w, h) {
    const { ch1, ch2, mode, voltDiv, timeDiv } = this.data;
    const sig = mode === 'CH1' ? ch1 : ch2;

    const gx = w / GRID_X;
    const gy = h / GRID_Y;
    // 每格代表 timeDiv 毫秒 -> 每像素代表的秒数
    const secPerPx = (timeDiv * 1e-3) / gx;
    // 每格代表 voltDiv 伏 -> 每伏对应的像素数
    const pxPerVolt = gy / voltDiv;

    // 静止波形（不滚动，便于读数），起点对齐屏幕左边
    ctx.strokeStyle = '#22ff5a';
    ctx.lineWidth = 2;
    ctx.shadowColor = 'rgba(34,255,90,0.6)';
    ctx.shadowBlur = 6;
    ctx.beginPath();
    for (let px = 0; px <= w; px++) {
      const t = px * secPerPx;
      const v = sig.A * Math.sin(2 * Math.PI * sig.f * t + sig.phi);
      const py = h / 2 - v * pxPerVolt;
      if (px === 0) ctx.moveTo(px, py); else ctx.lineTo(px, py);
    }
    ctx.stroke();
    ctx.shadowBlur = 0;

    // 超出屏幕提示
    const peakPx = sig.A * pxPerVolt;
    if (peakPx > h / 2) {
      ctx.fillStyle = 'rgba(255,120,0,0.9)';
      ctx.font = '12px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText('波形超出屏幕，请调大 V/div', w / 2, 32);
    }
  },

  drawLissajous(ctx, w, h) {
    const { ch1, ch2, voltDiv } = this.data;
    const gy = h / GRID_Y;
    const gx = w / GRID_X;
    const pxPerVoltY = gy / voltDiv;
    const pxPerVoltX = gx / voltDiv;

    ctx.strokeStyle = '#22ff5a';
    ctx.lineWidth = 2;
    ctx.shadowColor = 'rgba(34,255,90,0.6)';
    ctx.shadowBlur = 6;
    ctx.beginPath();

    // 画完整闭合曲线：取两频率的公共周期
    const fx = ch1.f, fy = ch2.f;
    const g = this.gcd(Math.round(fx), Math.round(fy)) || 1;
    const period = 1 / g;
    const N = 1200;
    for (let i = 0; i <= N; i++) {
      const t = (period * i) / N;
      const x = ch1.A * Math.sin(2 * Math.PI * fx * t + ch1.phi);
      const y = ch2.A * Math.sin(2 * Math.PI * fy * t + ch2.phi);
      const px = w / 2 + x * pxPerVoltX;
      const py = h / 2 - y * pxPerVoltY;
      if (i === 0) ctx.moveTo(px, py); else ctx.lineTo(px, py);
    }
    ctx.stroke();
    ctx.shadowBlur = 0;
  },

  gcd(a, b) {
    while (b) { [a, b] = [b, a % b]; }
    return a;
  },

  // ===== 读数（学生填格数，自动换算物理量） =====
  onGridInput(e) {
    const { field } = e.currentTarget.dataset;
    this.setData({ [`readings.${field}`]: e.detail.value }, () => this.recalcDerived());
  },

  recalcDerived() {
    const { readings, voltDiv, timeDiv } = this.data;
    const d = { ch1_A: null, ch1_f: null, ch2_A: null, ch2_f: null };

    for (const ch of ['ch1', 'ch2']) {
      const vg = parseFloat(readings[`${ch}_vdiv`]);
      const tg = parseFloat(readings[`${ch}_tdiv`]);
      if (isFinite(vg) && vg > 0) {
        // 峰峰值 -> 振幅
        d[`${ch}_A`] = ((vg * voltDiv) / 2).toFixed(3);
      }
      if (isFinite(tg) && tg > 0) {
        const T = tg * timeDiv * 1e-3;
        d[`${ch}_f`] = (1 / T).toFixed(2);
      }
    }
    this.setData({ derived: d });
  },

  async submitReadings() {
    const { levelId, assignmentId, derived } = this.data;
    if (!derived.ch1_A || !derived.ch1_f || !derived.ch2_A || !derived.ch2_f) {
      wx.showToast({ title: '请先数格子填完 CH1/CH2 的幅度和周期', icon: 'none' });
      return;
    }

    this.setData({ submitting: true });
    try {
      const url = assignmentId ? `/assignments/${assignmentId}/submit` : '/progress/submit';
      const data = await request({
        url,
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 2,
          experiment: 'oscilloscope',
          readings: [
            { channel: 'CH1', f: parseFloat(derived.ch1_f), A: parseFloat(derived.ch1_A) },
            { channel: 'CH2', f: parseFloat(derived.ch2_f), A: parseFloat(derived.ch2_A) }
          ]
        }
      });

      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}\n历史最佳：${data.best_score}`,
        showCancel: false,
        success: () => {
          if (data.passed) wx.navigateBack();
        }
      });
    } catch (err) {
      console.error('[oscilloscope] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 自测 =====
  onQuizChoice(e) {
    this.setData({ [`quizAnswers.${e.currentTarget.dataset.idx}`]: Number(e.detail.value) });
  },

  onQuizJudge(e) {
    const { idx, val } = e.currentTarget.dataset;
    this.setData({ [`quizAnswers.${idx}`]: val === 'true' });
  },

  onQuizInput(e) {
    this.setData({ [`quizAnswers.${e.currentTarget.dataset.idx}`]: e.detail.value });
  },

  checkOne(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    const q = this.data.quiz[idx];
    const ans = this.data.quizAnswers[idx];
    if (ans === undefined || ans === '') {
      wx.showToast({ title: '请先作答', icon: 'none' });
      return;
    }
    const ok = checkAnswer(q, ans);
    const prev = this.data.quizState[idx];
    this.setData({
      [`quizState.${idx}`]: ok ? 'right' : (prev === 'hint' || prev === 'wrong' ? 'wrong' : 'hint')
    });
  },

  submitQuiz() {
    const { quiz, quizAnswers } = this.data;
    let correct = 0;
    const state = {};
    quiz.forEach((q, i) => {
      const ok = checkAnswer(q, quizAnswers[i]);
      if (ok) correct++;
      state[i] = ok ? 'right' : 'wrong';
    });
    const score = Math.round((correct / quiz.length) * 100);
    this.setData({ quizState: state, quizScore: score });
    wx.showModal({
      title: '自测完成',
      content: `答对 ${correct}/${quiz.length} 题，得分 ${score}`,
      showCancel: false
    });
  },

  resetQuiz() {
    this.setData({
      quiz: pickQuiz('oscilloscope'),
      quizAnswers: {},
      quizState: {},
      quizScore: null
    });
  }
});
