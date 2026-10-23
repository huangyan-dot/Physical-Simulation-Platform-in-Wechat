// pages/experiments/newton-iter/newton-iter.js - 牛顿迭代法求根（数值分析 L12）
// 契约（docs/api-contract.md 附·数值分析批次）：
//   config: { func, func_key, interval, x0, tolerance, max_iterations, expected_iterations }
//   readings: [{ root, iterations, x0 }]（root 必填；后端按 x0 模拟牛顿得真值与期望迭代数）
//   user_config: { func_key, x0 }（换题/换初值时后端按提交参数重算）
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { numFuncs, funcPresets, fmtNum, newtonStepLimit, parseNum } from '../../../utils/numlab';
import {
  computeInitialView, panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';
import { createStepPlayer, startRaf } from '../../../utils/anim';

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      func: 'x^3 - x - 1 = 0',
      func_key: 'cubic_1',
      interval: [1.0, 1.5],
      x0: 1.5,
      tolerance: 0.000001,
      max_iterations: 50,
      expected_iterations: 4
    },
    funcLabels: [],
    funcIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    funcLabel: '',
    tolText: '',
    fmtXn: '',
    fmtFx: '',
    fmtDx: '',
    rootText: '',
    overExpected: false,
    x0Text: '1.5',
    // 自定义最大迭代步数：maxStepsText 留空表示用默认值 stepLimit
    maxStepsText: '',
    stepLimit: 0,
    maxSteps: 0,
    stepsCustom: false,
    // 当前迭代状态
    x0: 1.5,
    xn: 1.5,
    iterations: 0,
    history: [],
    converged: false,
    root: null,
    submitting: false,
    // 自动分拍迭代进行中（按钮在「自动迭代 / 停止演示」间切换）
    autoPlaying: false,
    isPreview: false,
    // 图像视图
    fullscreen: false,
    fullW: 0,
    fullH: 0,
    fullCanvasH: 0,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  // 缓存的 canvas 节点和上下文（小画布 + 全屏画布）
  _canvas: null,
  _ctx: null,
  _fullCanvas: null,
  _fullCtx: null,
  // 图像视野（数据坐标）与手势状态
  _view: null,
  _geoSmall: null,
  _geoFull: null,
  _gest: null,
  _gestStartView: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      funcLabels: Object.keys(numFuncs).map(k => numFuncs[k].label)
    });
    this._gest = createGestureTracker();
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.refreshConfigDisplay();
      this.reset();
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#plotCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[newton-iter] canvas 节点获取失败');
          return;
        }
        this._canvas = res[0].node;
        this._ctx = this._canvas.getContext('2d');
        this.drawPlot();
      });
  },

  onUnload() {
    this._unloaded = true;
    this.stopAutoRun();
    this._canvas = null;
    this._ctx = null;
    this._fullCanvas = null;
    this._fullCtx = null;
  },

  // 初始化 canvas 尺寸（小画布 + 全屏画布）
  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    this.setData({
      canvasWidth: sysInfo.windowWidth,
      canvasHeight: Math.min(sysInfo.windowWidth * 0.9, 420),
      fullW: sysInfo.windowWidth,
      fullH: sysInfo.windowHeight,
      // 全屏画布高度扣掉上下工具栏（与 wxss 的 44px / 72px 严格对应）
      fullCanvasH: Math.max(120, sysInfo.windowHeight - 44 - 72),
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  // 加载实验配置（后端不可用时回退默认 cubic_1）
  async loadExperimentConfig(levelId) {
    // 代次守卫：学生若在配置返回前换题/改参数（或已退出页面），晚到的配置不再覆盖
    this._cfgGen = (this._cfgGen || 0) + 1;
    const gen = this._cfgGen;
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      if (this._unloaded || gen !== this._cfgGen) return;
      const data = res.data || res || {};
      if (data.config && data.config.func_key && numFuncs[data.config.func_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...data.config },
          funcIndex: Object.keys(numFuncs).indexOf(data.config.func_key)
        });
      }
    } catch (err) {
      console.warn('[newton-iter] 后端不可用，使用默认配置');
    }
    this.refreshConfigDisplay();
    this.reset();
  },

  // 刷新与 config 相关的展示字段
  refreshConfigDisplay() {
    const { config } = this.data;
    // 步数上限按函数的收敛特性给出，并受后端 max_iterations 约束
    // （后端校验 iterations∈[1, max_iterations] 且不可由 user_config 覆盖）
    const stepLimit = Math.min(newtonStepLimit(config.func_key), config.max_iterations);
    const maxSteps = this.data.stepsCustom
      ? Math.min(this.data.maxSteps, stepLimit)
      : stepLimit;
    this.setData({
      funcLabel: (numFuncs[config.func_key] || numFuncs.cubic_1).label,
      tolText: fmtNum(config.tolerance),
      stepLimit,
      maxSteps,
      maxStepsText: this.data.stepsCustom ? String(maxSteps) : ''
    });
  },

  // 输入框中的步数：留空=默认上限；否则须为 0 ~ stepLimit 的整数
  onMaxStepsInput(e) {
    this.setData({ maxStepsText: e.detail.value });
  },

  // 解析自定义步数。非法时提示并返回 null
  parseMaxSteps() {
    const limit = this.data.stepLimit;
    const txt = String(this.data.maxStepsText || '').trim();
    if (txt === '') return { value: limit, custom: false };
    if (!/^\d+$/.test(txt)) {
      wx.showToast({ title: `迭代步数须为 0 ~ ${limit} 的整数`, icon: 'none' });
      return null;
    }
    const n = parseInt(txt, 10);
    if (n > limit) {
      wx.showToast({ title: `本题迭代步数最多 ${limit} 步`, icon: 'none' });
      return null;
    }
    return { value: n, custom: true };
  },

  // 重置迭代状态回初值 x0（视野一并复位）
  reset() {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    this.stopAutoRun();
    const x0 = Number(this.data.config.x0);
    this._view = this.computeView();
    // 步数设为 0：不迭代，直接以 x₀ 为读数（便于观察步数与精度的关系）
    if (this.data.maxSteps === 0) {
      this.applyState(x0, 0, [], true, x0);
    } else {
      this.applyState(x0, 0, [], false, null);
    }
    this.setData({ x0, x0Text: String(Number(x0.toPrecision(6))) });
  },

  // 统一状态入口：更新当前迭代点/步数/记录并重绘
  applyState(xn, iterations, history, converged, root) {
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const lastDx = history.length > 0 ? history[history.length - 1].dxNum : null;
    this.setData({
      xn,
      iterations,
      history,
      converged,
      root,
      fmtXn: fmtNum(xn),
      fmtFx: fmtNum(fd.f(xn)),
      fmtDx: lastDx === null ? '-' : fmtNum(lastDx),
      rootText: root === null ? '' : fmtNum(root),
      overExpected: converged && iterations > this.data.config.expected_iterations + 2
    });
    this.drawPlot();
    if (converged) this.startPulse(); else this.stopPulse();
  },

  // 收敛后的根点脉冲：rAF 持续重绘扩张光环；重开/卸载时停止
  startPulse() {
    const cv = this.data.fullscreen ? this._fullCanvas : this._canvas;
    if (this._pulseRaf || !cv) return;
    this._pulseT0 = Date.now();
    this._pulseRaf = startRaf(cv, () => {
      if (this._unloaded) return false;
      this._pulseT = Date.now() - this._pulseT0;
      this.drawPlot(); // 内部按 fullscreen 只画可见的一块画布
      return true;
    });
  },

  stopPulse() {
    if (this._pulseRaf) {
      this._pulseRaf.cancel();
      this._pulseRaf = null;
    }
    this._pulseT = 0;
  },

  // 换函数（换题）：用预设初值重开，提交时随 user_config 上报
  onFuncChange(e) {
    const idx = Number(e.detail.value);
    const keys = Object.keys(numFuncs);
    const key = keys[idx];
    const preset = funcPresets[key];
    this.setData({
      funcIndex: idx,
      config: this.configReplace(key, preset)
    });
    this.refreshConfigDisplay();
    this.reset();
  },

  // 构造换题后的 config（不直接改 this.data）
  configReplace(key, preset) {
    const cfg = { ...this.data.config };
    cfg.func_key = key;
    cfg.interval = [...preset.interval];
    cfg.x0 = preset.x0;
    return cfg;
  },

  // 初值输入框
  onX0Input(e) {
    this.setData({ x0Text: e.detail.value });
  },

  // 从 x0 重新开始（应用输入框中的初值与步数上限）
  restartFromX0() {
    const v = parseNum(this.data.x0Text);
    if (v === null) {
      wx.showToast({ title: '初值无效，请输入数字', icon: 'none' });
      return;
    }
    const steps = this.parseMaxSteps();
    if (!steps) return;
    this.setData({
      config: { ...this.data.config, x0: v },
      maxSteps: steps.value,
      stepsCustom: steps.custom
    });
    this.reset();
  },

  // 执行一步牛顿迭代：x_{n+1} = x_n − f(x_n)/f'(x_n)；
  // 返回 true 表示推进了一步，false 表示被拦截（f′=0/发散/已收敛），自动播放器据此停机
  step() {
    if (this.data.converged) return false;
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const { xn, iterations, history, config } = this.data;
    const fx = fd.f(xn);
    const dfx = fd.df(xn);

    if (dfx === 0) {
      wx.showToast({ title: 'f\'(x)=0，切线水平，请更换初值', icon: 'none' });
      return false;
    }

    const xNext = xn - fx / dfx;
    if (!isFinite(xNext)) {
      wx.showToast({ title: '迭代发散，请更换初值重试', icon: 'none' });
      return false;
    }
    const dx = Math.abs(xNext - xn);
    const n = iterations + 1;
    const newHistory = history.concat([{
      n,
      x: fmtNum(xn),
      fx: fmtNum(fx),
      dx: fmtNum(dx),
      dxNum: dx,
      xv: xn // 原始数值供画布重绘历史切线阶梯
    }]);

    // 停止准则与后端一致：|x_{n+1} − x_n| < tolerance
    if (dx < config.tolerance) {
      this.applyState(xNext, n, newHistory, true, xNext);
      wx.showToast({ title: '已收敛，可提交成绩', icon: 'success' });
    } else if (n >= this.data.maxSteps) {
      this.applyState(xNext, n, newHistory, true, xNext);
      wx.showToast({ title: `已达步数上限 ${this.data.maxSteps}`, icon: 'none' });
    } else {
      this.applyState(xNext, n, newHistory, false, null);
    }
  },

  // 自动迭代 / 停止演示：一拍拍画出切线阶梯，直到收敛（f′=0 等拦截时自动停机）
  autoRun() {
    if (this._player) {
      this.stopAutoRun();
      return;
    }
    if (this.data.converged) return;
    this.setData({ autoPlaying: true });
    this._player = createStepPlayer({
      interval: 480, // 牛顿切线需要时间读，每拍略慢
      maxSteps: (Number(this.data.maxSteps) || 50) + 2,
      isDone: () => this.data.converged,
      step: () => this.step(),
      onEnd: () => {
        this._player = null;
        if (this.data.autoPlaying) this.setData({ autoPlaying: false });
      }
    });
  },

  // 停止分拍演示（换初值/换题/卸载时也会调用）
  stopAutoRun() {
    const p = this._player;
    this._player = null;
    if (p) p.cancel();
    this.stopPulse();
    if (this.data.autoPlaying) this.setData({ autoPlaying: false });
  },

  // 提交读数评分
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, root, iterations, converged, x0, config } = this.data;
    if (!converged || root === null) {
      wx.showToast({ title: '请先迭代至收敛', icon: 'none' });
      return;
    }
    this.setData({ submitting: true });
    try {
      // 后端校验 iterations∈[1, max_iterations]，且该字段为可选（缺省不罚迭代分）：
      // 步数上限设为 0（未迭代）时按契约省略，避免 400
      const reading = { root: Number(root), x0: Number(x0) };
      if (iterations > 0) reading.iterations = iterations;
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 12,
          experiment: 'newton_iter',
          readings: [reading],
          // 携带实际出题参数：换题/换初值时后端按此重算真值与期望迭代数（契约 §12）
          user_config: {
            func_key: config.func_key,
            x0: Number(x0)
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
      console.error('[newton-iter] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // 初始视野：区间与初值（含当前迭代点）的并集外扩 15%
  computeView() {
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const { config, x0, xn } = this.data;
    const lo = Math.min(config.interval[0], config.interval[1], x0, xn);
    const hi = Math.max(config.interval[0], config.interval[1], x0, xn);
    return computeInitialView(fd.f, lo, hi);
  },

  // 复位视野（双击或换题/换初值时调用）
  resetView() {
    this._view = this.computeView();
    this.drawPlot();
  },

  // 按钮缩放：以当前视野中心为不动点（模拟器鼠标无法双指捏合时的替代操作）
  zoomBy(e) {
    const factor = Number(e.currentTarget.dataset.factor);
    if (!(factor > 0)) return;
    if (!this._view) this._view = this.computeView();
    const center = {
      x: (this._view.xMin + this._view.xMax) / 2,
      y: (this._view.yMin + this._view.yMax) / 2
    };
    this._view = clampView(zoomView(this._view, center, factor));
    this.drawPlot();
  },

  // 进入全屏：懒加载全屏 canvas 节点后按当前视野重绘
  enterFullscreen() {
    this.setData({ fullscreen: true }, () => {
      if (this._fullCanvas) {
        if (this._pulseRaf) this._pulseRaf.setCanvas(this._fullCanvas); // 脉冲循环挂到可见画布
        this.drawPlot();
        return;
      }
      const query = wx.createSelectorQuery().in(this);
      query.select('#fullCanvas')
        .fields({ node: true, size: true })
        .exec((res) => {
          if (!res[0]) {
            console.error('[newton-iter] 全屏 canvas 节点获取失败');
            return;
          }
          this._fullCanvas = res[0].node;
          this._fullCtx = this._fullCanvas.getContext('2d');
          if (this._pulseRaf) this._pulseRaf.setCanvas(this._fullCanvas);
          this.drawPlot();
        });
    });
  },

  exitFullscreen() {
    // 全屏期间小画布跳过重绘，收回后补画一帧；脉冲循环切回小画布节点
    this.setData({ fullscreen: false }, () => {
      if (this._pulseRaf && this._canvas) this._pulseRaf.setCanvas(this._canvas);
      this.drawPlot();
    });
  },

  // 画布触摸：小画布单击进全屏；两块画布均可缩放/平移；双击复位
  onPlotTouchStart(e) {
    const mode = this._gest.anchor(e.touches);
    if (mode) this._gestStartView = { ...this._view };
  },

  onPlotTouchMove(e) {
    const g = this._gest.move(e.touches);
    if (!g || !this._gestStartView) return;
    const geo = e.currentTarget.id === 'fullCanvas' ? this._geoFull : this._geoSmall;
    if (!geo) return;
    if (g.type === 'pan') {
      const dxData = -(g.dxPx / geo.pw) * (this._gestStartView.xMax - this._gestStartView.xMin);
      const dyData = (g.dyPx / geo.ph) * (this._gestStartView.yMax - this._gestStartView.yMin);
      this._view = clampView(panView(this._gestStartView, dxData, dyData));
    } else {
      const center = pxToData(this._gestStartView, geo, g.cx, g.cy);
      this._view = clampView(zoomView(this._gestStartView, center, g.ratio));
    }
    this.drawPlot();
  },

  onPlotTouchEnd(e) {
    if (e.touches && e.touches.length > 0) {
      // 手指未全部离开：对剩余触点重新锚定（捏合转拖动等场景）
      const mode = this._gest.anchor(e.touches);
      if (mode) this._gestStartView = { ...this._view };
      return;
    }
    const t = this._gest.end();
    const isSmall = e.currentTarget.id === 'plotCanvas';
    if (t === 'tap' && isSmall) {
      this.enterFullscreen();
    } else if (t === 'doubletap') {
      // 页内双击复位视野；全屏下双击退出，作为按钮被遮挡时的保底出口
      if (isSmall) this.resetView();
      else this.exitFullscreen();
    }
  },

  // 绘制：非全屏只画小画布；全屏时只画全屏画布（隐藏画布跳过重绘）
  drawPlot() {
    // 迭代点跑出视野时自动扩展（发散/大步跳出可见区）
    if (this._view) {
      const { xn } = this.data;
      if (xn < this._view.xMin || xn > this._view.xMax) {
        this._view = this.computeView();
      }
    }
    if (!this.data.fullscreen && this._canvas && this._ctx) {
      this._drawOn(this._canvas, this._ctx, this.data.canvasWidth, this.data.canvasHeight, 'small');
    }
    if (this.data.fullscreen && this._fullCanvas && this._fullCtx) {
      this._drawOn(this._fullCanvas, this._fullCtx, this.data.fullW, this.data.fullCanvasH, 'full');
    }
  },

  // 在指定 canvas 上绘制函数曲线、坐标轴刻度、迭代点与切线
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;

    // backing store 仅在尺寸变化时重设（每帧重赋 width 会清空画布状态且有开销）
    const pr = this.data.pixelRatio || 1;
    const bw = Math.round(w * pr), bh = Math.round(h * pr);
    if (canvas.width !== bw || canvas.height !== bh) {
      canvas.width = bw;
      canvas.height = bh;
    }
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const { config, x0, xn, converged, root } = this.data;

    // 布局边距
    const mL = 52, mR = 16, mT = 16, mB = 36;
    const pw = w - mL - mR;
    const ph = h - mT - mB;
    if (pw <= 0 || ph <= 0) return;
    const geo = { mL, mT, pw, ph };
    if (which === 'full') this._geoFull = geo;
    else this._geoSmall = geo;

    const X = (x) => mL + (x - view.xMin) / (view.xMax - view.xMin) * pw;
    const Y = (y) => mT + (view.yMax - y) / (view.yMax - view.yMin) * ph;
    const clampPy = (py) => Math.max(mT - 1000, Math.min(mT + ph + 1000, py));
    const clampX = (px) => Math.max(mL + 24, Math.min(mL + pw - 24, px));

    // 背景
    ctx.fillStyle = '#10182a';
    ctx.fillRect(0, 0, w, h);

    // 刻度网格与刻度值（1-2-5 步长）
    ctx.font = '10px sans-serif';
    ctx.lineWidth = 1;
    const stepX = niceStep((view.xMax - view.xMin) / 5);
    ctx.textAlign = 'center';
    for (let i = Math.ceil(view.xMin / stepX); i * stepX <= view.xMax; i++) {
      const x = i * stepX;
      const px = X(x);
      if (px < mL - 2 || px > mL + pw + 2) continue;
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
      ctx.beginPath();
      ctx.moveTo(px, mT);
      ctx.lineTo(px, mT + ph);
      ctx.stroke();
      ctx.fillStyle = 'rgba(255, 255, 255, 0.45)';
      ctx.fillText(fmtTick(x), px, mT + ph + 24);
    }
    const stepY = niceStep((view.yMax - view.yMin) / 4);
    ctx.textAlign = 'right';
    for (let i = Math.ceil(view.yMin / stepY); i * stepY <= view.yMax; i++) {
      const y = i * stepY;
      const py = Y(y);
      if (py < mT - 2 || py > mT + ph + 2) continue;
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
      ctx.beginPath();
      ctx.moveTo(mL, py);
      ctx.lineTo(mL + pw, py);
      ctx.stroke();
      ctx.fillStyle = 'rgba(255, 255, 255, 0.45)';
      ctx.fillText(fmtTick(y), mL - 6, py + 3);
    }

    // 坐标轴（y=0 / x=0 在视野内时加粗）
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.4)';
    if (view.yMin <= 0 && view.yMax >= 0) {
      ctx.beginPath();
      ctx.moveTo(mL, Y(0));
      ctx.lineTo(mL + pw, Y(0));
      ctx.stroke();
    }
    if (view.xMin <= 0 && view.xMax >= 0) {
      ctx.beginPath();
      ctx.moveTo(X(0), mT);
      ctx.lineTo(X(0), mT + ph);
      ctx.stroke();
    }

    // 函数曲线（离屏过远的 y 夹取，防 canvas 精度问题）
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2.5;
    ctx.beginPath();
    const N = 240;
    let started = false;
    for (let i = 0; i <= N; i++) {
      const x = view.xMin + (view.xMax - view.xMin) * i / N;
      const y = fd.f(x);
      if (!isFinite(y)) {
        started = false;
        continue;
      }
      if (!started) {
        ctx.moveTo(X(x), clampPy(Y(y)));
        started = true;
      } else {
        ctx.lineTo(X(x), clampPy(Y(y)));
      }
    }
    ctx.stroke();

    // 文字标注
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'center';
    const yAxisPy = Math.min(Math.max(Y(0), mT + 10), mT + ph - 10);

    const fxn = fd.f(xn);
    const dfxn = fd.df(xn);

    // 历史切线阶梯：曲线上的点沿切线落到 x 轴，再竖直回到曲线——牛顿法的之字路
    const clampPx = (px) => Math.max(mL - 80, Math.min(mL + pw + 80, px));
    (this.data.history || []).forEach((h2) => {
      if (typeof h2.xv !== 'number') return;
      const xi = h2.xv;
      const fi = fd.f(xi);
      const di = fd.df(xi);
      if (!isFinite(fi) || di === 0) return;
      const nx = xi - fi / di;
      if (!isFinite(nx)) return;
      ctx.strokeStyle = 'rgba(231, 76, 60, 0.28)';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(clampPx(X(xi)), clampPy(Y(fi)));
      ctx.lineTo(clampPx(X(nx)), yAxisPy);
      ctx.stroke();
      const fy = fd.f(nx);
      if (isFinite(fy)) {
        ctx.strokeStyle = 'rgba(243, 156, 18, 0.3)';
        ctx.beginPath();
        ctx.moveTo(clampPx(X(nx)), yAxisPy);
        ctx.lineTo(clampPx(X(nx)), clampPy(Y(fy)));
        ctx.stroke();
      }
      ctx.fillStyle = 'rgba(46, 204, 113, 0.55)';
      ctx.beginPath();
      ctx.arc(Math.max(mL, Math.min(mL + pw, X(nx))), yAxisPy, 2.4, 0, Math.PI * 2);
      ctx.fill();
    });

    // 当前迭代点 x_n：竖直虚线 + 曲线上的点
    ctx.strokeStyle = '#f39c12';
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    ctx.moveTo(X(xn), mT);
    ctx.lineTo(X(xn), mT + ph);
    ctx.stroke();
    ctx.fillStyle = '#f39c12';
    ctx.beginPath();
    ctx.arc(X(xn), clampPy(Y(fxn)), 4, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillText('x=' + fmtNum(xn, 5), clampX(X(xn)), mT + 12);

    // 切线及与 x 轴交点 x_{n+1}（收敛后只标根）
    if (!converged && dfxn !== 0) {
      const xNext = xn - fxn / dfxn;
      // 切线：y = f(xn) + f'(xn)(x − xn)，画满绘图区（自动裁剪）
      ctx.strokeStyle = '#e74c3c';
      ctx.setLineDash([]);
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(X(view.xMin), clampPy(Y(fxn + dfxn * (view.xMin - xn))));
      ctx.lineTo(X(view.xMax), clampPy(Y(fxn + dfxn * (view.xMax - xn))));
      ctx.stroke();

      // x_{n+1}：竖直虚线 + x 轴上的点
      ctx.strokeStyle = '#2ecc71';
      ctx.setLineDash([4, 4]);
      ctx.beginPath();
      ctx.moveTo(X(xNext), mT);
      ctx.lineTo(X(xNext), mT + ph);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.fillStyle = '#2ecc71';
      ctx.beginPath();
      ctx.arc(X(xNext), yAxisPy, 4, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillText('x' + (this.data.iterations + 1) + '=' + fmtNum(xNext, 5), clampX(X(xNext)), mT + ph - 20);
    }

    // 收敛后标出求得的根：容差带 + 脉冲光环 + 根线/根点
    if (converged && root !== null) {
      const half = Math.max(this.data.config.tolerance / 2, 1e-12);
      const tx1 = Math.max(X(root - half), mL);
      const tx2 = Math.min(X(root + half), mL + pw);
      if (tx2 > tx1) {
        ctx.fillStyle = 'rgba(46, 204, 113, 0.16)';
        ctx.fillRect(tx1, mT, tx2 - tx1, ph);
        ctx.font = '9px sans-serif';
        ctx.textAlign = 'left';
        ctx.fillStyle = 'rgba(46, 204, 113, 0.8)';
        ctx.fillText('容差 ±' + fmtNum(half, 2), tx1 + 3, mT + ph - 4);
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
      }
      const sec = (this._pulseT || 0) / 1000;
      for (let k = 0; k < 2; k++) {
        const phase = ((sec + k * 0.8) % 1.6) / 1.6;
        ctx.strokeStyle = `rgba(46, 204, 113, ${(0.75 * (1 - phase)).toFixed(3)})`;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(X(root), yAxisPy, 6 + phase * 26, 0, Math.PI * 2);
        ctx.stroke();
      }
      ctx.strokeStyle = '#2ecc71';
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(X(root), mT);
      ctx.lineTo(X(root), mT + ph);
      ctx.stroke();
      ctx.fillStyle = '#2ecc71';
      ctx.beginPath();
      ctx.arc(X(root), yAxisPy, 5, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillText('root=' + fmtNum(root, 6), clampX(X(root)), mT + ph - 20);
    }

    ctx.setLineDash([]);

    // 函数式画在画布内（原生 canvas 上的 view 可能被遮挡，画进来最稳）
    ctx.textAlign = 'left';
    ctx.font = '12px sans-serif';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.75)';
    ctx.fillText(this.data.funcLabel, mL + 4, mT + 14);
    if (which === 'full') {
      ctx.textAlign = 'right';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
      ctx.fillText('双击图像退出全屏', mL + pw, mT + 14);
    }
  }
});
