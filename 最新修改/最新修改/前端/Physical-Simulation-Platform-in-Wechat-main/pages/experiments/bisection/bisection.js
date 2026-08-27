// pages/experiments/bisection/bisection.js - 二分法求根（数值分析 L11）
// 契约（docs/api-contract.md 附·数值分析批次）：
//   config: { func, func_key, interval, tolerance, max_iterations, expected_iterations }
//   readings: [{ root, iterations }]（root 必填）
//   user_config: { func_key, interval }（换题/换区间时后端按提交参数重算真值根）
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { numFuncs, funcPresets, fmtNum, bisectStepLimit } from '../../../utils/numlab';
import {
  computeInitialView, panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      func: 'x^3 - x - 1 = 0',
      func_key: 'cubic_1',
      interval: [1.0, 1.5],
      tolerance: 0.000001,
      max_iterations: 50,
      expected_iterations: 19
    },
    funcLabels: [],
    funcIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    funcLabel: '',
    initialA: '',
    initialB: '',
    tolText: '',
    fmtA: '',
    fmtB: '',
    fmtMid: '',
    fmtFmid: '',
    fmtWidth: '',
    rootText: '',
    // 自定义区间输入（默认取 config.interval）
    inputA: '',
    inputB: '',
    // 自定义最大迭代步数：maxStepsText 留空表示用默认值 stepLimit
    maxStepsText: '',
    stepLimit: 0,
    maxSteps: 0,
    stepsCustom: false,
    // 当前二分状态
    a: 1.0,
    b: 1.5,
    mid: 1.25,
    iterations: 0,
    history: [],
    finished: false,
    root: null,
    submitting: false,
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
          console.error('[bisection] canvas 节点获取失败');
          return;
        }
        this._canvas = res[0].node;
        this._ctx = this._canvas.getContext('2d');
        this.drawPlot();
      });
  },

  onUnload() {
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
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      const data = res.data || res || {};
      if (data.config && data.config.func_key && numFuncs[data.config.func_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...data.config },
          funcIndex: Object.keys(numFuncs).indexOf(data.config.func_key)
        });
      }
    } catch (err) {
      console.warn('[bisection] 后端不可用，使用默认配置');
    }
    this.refreshConfigDisplay();
    this.reset();
  },

  // 刷新与 config 相关的展示字段
  refreshConfigDisplay() {
    const { config } = this.data;
    // 步数上限随函数与当前区间变化，并受后端 max_iterations 约束；
    // 自定义值超出新上限时就地夹取（例如换题后区间变窄）
    const stepLimit = Math.min(
      bisectStepLimit(config.interval, config.tolerance),
      config.max_iterations
    );
    const maxSteps = this.data.stepsCustom
      ? Math.min(this.data.maxSteps, stepLimit)
      : stepLimit;
    this.setData({
      funcLabel: (numFuncs[config.func_key] || numFuncs.cubic_1).label,
      initialA: fmtNum(config.interval[0]),
      initialB: fmtNum(config.interval[1]),
      inputA: String(Number(Number(config.interval[0]).toPrecision(6))),
      inputB: String(Number(Number(config.interval[1]).toPrecision(6))),
      tolText: fmtNum(config.tolerance),
      stepLimit,
      maxSteps,
      maxStepsText: this.data.stepsCustom ? String(maxSteps) : ''
    });
  },

  // 输入框中的步数：留空=默认上限；否则须为 0 ~ limit 的整数
  onMaxStepsInput(e) {
    this.setData({ maxStepsText: e.detail.value });
  },

  // 解析自定义步数，limit 为按新区间算出的上限。非法时提示并返回 null
  parseMaxSteps(limit) {
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

  // 重置二分状态回初始区间（视野一并复位）
  reset() {
    const [a, b] = this.data.config.interval;
    this._view = this.computeView();
    // 步数设为 0：不迭代，直接以区间中点为读数（便于观察步数与精度的关系）
    if (this.data.maxSteps === 0) {
      this.applyState(a, b, 0, [], true, (a + b) / 2);
      return;
    }
    this.applyState(a, b, 0, [], false, null);
  },

  // 统一状态入口：更新区间/步数/记录并重绘
  applyState(a, b, iterations, history, finished, root) {
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const mid = (a + b) / 2;
    this.setData({
      a,
      b,
      mid,
      iterations,
      history,
      finished,
      root,
      fmtA: fmtNum(a),
      fmtB: fmtNum(b),
      fmtMid: fmtNum(mid),
      fmtFmid: fmtNum(fd.f(mid)),
      fmtWidth: fmtNum(b - a),
      rootText: root === null ? '' : fmtNum(root)
    });
    this.drawPlot();
  },

  // 换函数（换题）：用预设区间重开，提交时随 user_config 上报
  onFuncChange(e) {
    const idx = Number(e.detail.value);
    const keys = Object.keys(numFuncs);
    const key = keys[idx];
    const preset = funcPresets[key];
    this.setData({
      funcIndex: idx,
      config: { ...this.data.config, func_key: key, interval: [...preset.interval] }
    });
    this.refreshConfigDisplay();
    this.reset();
  },

  // ===== 自定义区间输入 =====
  onInputA(e) {
    this.setData({ inputA: e.detail.value });
  },

  onInputB(e) {
    this.setData({ inputB: e.detail.value });
  },

  // 应用自定义区间：校验 a<b 且端点变号，通过后重开求解
  applyInterval() {
    const a = parseFloat(this.data.inputA);
    const b = parseFloat(this.data.inputB);
    if (!isFinite(a) || !isFinite(b)) {
      wx.showToast({ title: '区间端点须为数字', icon: 'none' });
      return;
    }
    if (a >= b) {
      wx.showToast({ title: '须满足 a < b', icon: 'none' });
      return;
    }
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const fa = fd.f(a);
    const fb = fd.f(b);
    if (fa * fb > 0) {
      wx.showToast({ title: '端点不变号：f(a)·f(b)>0，区间内可能无根', icon: 'none' });
      return;
    }
    if (fa === 0 || fb === 0) {
      wx.showToast({ title: '端点恰好是根，请微调区间', icon: 'none' });
      return;
    }
    // 步数上限依赖区间宽度：先按新区间算出上限，再校验输入的步数
    const limit = Math.min(
      bisectStepLimit([a, b], this.data.config.tolerance),
      this.data.config.max_iterations
    );
    const steps = this.parseMaxSteps(limit);
    if (!steps) return;
    this.setData({
      config: { ...this.data.config, interval: [a, b] },
      stepLimit: limit,
      maxSteps: steps.value,
      stepsCustom: steps.custom
    });
    this.refreshConfigDisplay();
    this.reset();
    wx.showToast({
      title: steps.custom ? `已应用：区间 + 上限 ${steps.value} 步` : '参数已更新',
      icon: 'success'
    });
  },

  // 学生点击保留哪半区间（二分核心操作）
  pickSide(e) {
    this.step(e.currentTarget.dataset.side);
  },

  // 执行一步二分：side = 'left' 保留 [a, c]，'right' 保留 [c, b]
  step(side) {
    if (this.data.finished) return;
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const { a, b, mid, iterations, history, config } = this.data;
    const fa = fd.f(a);
    const fc = fd.f(mid);
    const newA = side === 'left' ? a : mid;
    const newB = side === 'left' ? mid : b;
    const nfa = side === 'left' ? fa : fc;
    const nfb = side === 'left' ? fc : fd.f(b);

    // 教学引导：保留的半区间必须变号，否则不含根
    if (nfa * nfb > 0) {
      wx.showToast({ title: '该半区间不变号，根不在其中', icon: 'none' });
      return;
    }

    const n = iterations + 1;
    const newHistory = history.concat([{
      n,
      a: fmtNum(a),
      b: fmtNum(b),
      c: fmtNum(mid),
      fc: fmtNum(fc),
      width: fmtNum(newB - newA)
    }]);

    if (newB - newA < config.tolerance) {
      const root = (newA + newB) / 2;
      this.applyState(newA, newB, n, newHistory, true, root);
      wx.showToast({ title: '已收敛，可提交成绩', icon: 'success' });
    } else if (n >= this.data.maxSteps) {
      this.applyState(newA, newB, n, newHistory, true, (newA + newB) / 2);
      wx.showToast({ title: `已达步数上限 ${this.data.maxSteps}`, icon: 'none' });
    } else {
      this.applyState(newA, newB, n, newHistory, false, null);
    }
  },

  // 自动演示：替学生选变号的半区间直到收敛
  autoRun() {
    if (this.data.finished) return;
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    let guard = 0;
    while (!this.data.finished && guard < 100) {
      guard++;
      const { a, mid } = this.data;
      this.step(fd.f(a) * fd.f(mid) < 0 ? 'left' : 'right');
    }
  },

  // 重新开始（保留当前函数与区间）
  restart() {
    this.reset();
  },

  // 提交读数评分
  async submitReadings() {
    const { levelId, root, iterations, finished, config } = this.data;
    if (!finished || root === null) {
      wx.showToast({ title: '请先完成二分求解', icon: 'none' });
      return;
    }
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 11,
          experiment: 'bisection',
          readings: [{ root: Number(root), iterations }],
          // 携带实际出题参数：换题/换区间时后端按此重算真值根（契约 §12）
          user_config: {
            func_key: config.func_key,
            interval: config.interval
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
      console.error('[bisection] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // 初始视野：初始区间外扩 15%，y 自适应
  computeView() {
    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const [ia, ib] = this.data.config.interval;
    return computeInitialView(fd.f, ia, ib);
  },

  // 复位视野（双击或换题/换区间时调用）
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
        this.drawPlot();
        return;
      }
      const query = wx.createSelectorQuery().in(this);
      query.select('#fullCanvas')
        .fields({ node: true, size: true })
        .exec((res) => {
          if (!res[0]) {
            console.error('[bisection] 全屏 canvas 节点获取失败');
            return;
          }
          this._fullCanvas = res[0].node;
          this._fullCtx = this._fullCanvas.getContext('2d');
          this.drawPlot();
        });
    });
  },

  exitFullscreen() {
    this.setData({ fullscreen: false });
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

  // 绘制：小画布始终绘制；全屏时同步绘制全屏画布
  drawPlot() {
    if (!this._canvas || !this._ctx) return;
    this._drawOn(this._canvas, this._ctx, this.data.canvasWidth, this.data.canvasHeight, 'small');
    if (this.data.fullscreen && this._fullCanvas && this._fullCtx) {
      this._drawOn(this._fullCanvas, this._fullCtx, this.data.fullW, this.data.fullCanvasH, 'full');
    }
  },

  // 在指定 canvas 上绘制函数曲线、坐标轴刻度与当前二分状态
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;

    canvas.width = w * this.data.pixelRatio;
    canvas.height = h * this.data.pixelRatio;
    ctx.setTransform(this.data.pixelRatio, 0, 0, this.data.pixelRatio, 0, 0);

    const fd = numFuncs[this.data.config.func_key] || numFuncs.cubic_1;
    const { config, a, b, mid, finished, root } = this.data;

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

    // 当前二分区间 [a, b] 高亮（裁剪到绘图区）
    ctx.fillStyle = 'rgba(52, 152, 219, 0.12)';
    const ax = Math.max(X(a), mL);
    const bx = Math.min(X(b), mL + pw);
    if (bx > ax) ctx.fillRect(ax, mT, bx - ax, ph);

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
      const py = Math.max(mT - 1000, Math.min(mT + ph + 1000, Y(y)));
      if (!started) {
        ctx.moveTo(X(x), py);
        started = true;
      } else {
        ctx.lineTo(X(x), py);
      }
    }
    ctx.stroke();

    // 文字标注
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'center';
    const yAxisPy = Math.min(Math.max(Y(0), mT + 10), mT + ph - 10);

    // 区间端点 a / b 标记
    ctx.fillStyle = 'rgba(255, 255, 255, 0.9)';
    [a, b].forEach((v) => {
      ctx.beginPath();
      ctx.arc(X(v), yAxisPy, 3, 0, Math.PI * 2);
      ctx.fill();
    });
    ctx.fillText('a=' + fmtNum(a, 5), clampX(X(a)), mT + ph - 20);
    ctx.fillText('b=' + fmtNum(b, 5), clampX(X(b)), mT + ph - 20);

    // 中点 c：竖直虚线 + 曲线上交点
    ctx.strokeStyle = '#f39c12';
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    ctx.moveTo(X(mid), mT);
    ctx.lineTo(X(mid), mT + ph);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = '#f39c12';
    ctx.beginPath();
    ctx.arc(X(mid), Math.max(mT - 1000, Math.min(mT + ph + 1000, Y(fd.f(mid)))), 4, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillText('c=' + fmtNum(mid, 5), clampX(X(mid)), mT + 12);

    // 收敛后标出求得的根
    if (finished && root !== null) {
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
      ctx.fillText('root=' + fmtNum(root, 6), clampX(X(root)), mT + ph - 40);
    }

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
