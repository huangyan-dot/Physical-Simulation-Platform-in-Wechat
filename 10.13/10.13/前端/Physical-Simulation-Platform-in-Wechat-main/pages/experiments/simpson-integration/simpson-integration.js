// pages/experiments/simpson-integration/simpson-integration.js - 复合辛普森积分法（数值分析 L17）
// 契约（docs/api-contract.md 附·数值分析批次 L7）：
//   config: { problem_key, problem_desc, func_desc, a, b, n, n_hint }
//   readings: [{ value, odd_sum?, even_sum? }]（value=学生算出的 S_n，必填；
//             odd_sum/even_sum=奇/偶下标节点函数值和，选填仅记入审计不参与打分）
//   user_config: { problem_key, a, b, n }（换题/换区间/换 n，后端按提交参数重算真值 S_n）
// 教学设计：页面演示到「节点表 + h + 公式结构」，奇/偶求和与加权组合留给学生手算填 S_n；
// 真值是「方法的输出」S_n 而非精确积分 I——抄 I 被截断误差压住（n=2 抄 I=2 约 42 分不过线）。
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { integProblems, lagrangeP, fmtNum, parseNum, parseIntStrict } from '../../../utils/numlab';
import {
  panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';
import { tween, easeLinear } from '../../../utils/anim';

const PROBLEM_KEYS = Object.keys(integProblems);

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      problem_key: 'sin_pi',
      a: 0,
      b: 3.141592653589793,
      n: 2,
      n_hint: 'n 为偶数子区间数，加大 n 观察 S_n 逼近精确积分'
    },
    problemLabels: [],
    problemIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    problemLabel: '',
    problemDesc: '',
    fLabel: '',
    // 节点表（i / x_i / f(x_i) / 权重系数 4 或 2）
    nodeRows: [],
    nodeRowsTail: [],
    nodeTableTruncated: false,
    // 生效参数（input 文本 + 已应用的数值）
    aText: '',
    bText: '',
    aApplied: 0,
    bApplied: 0,
    nText: '',
    nApplied: 2,
    // 公式卡（数值代入展示，求和与加权留给学生）
    hLabel: '',
    fALabel: '',
    fBLabel: '',
    oddCount: 0,
    evenCount: 0,
    // 奇/偶和输入（选填，仅审计）与最终答案
    oddSumText: '',
    evenSumText: '',
    valueText: '',
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

  // 当前题库对象（非 data，避免大对象序列化）
  _prob: null,
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
  // 抛物面板逐块揭示（0..1），换题/换参数时从左向右长出
  _panelReveal: 1,
  _revealTween: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      problemLabels: PROBLEM_KEYS.map(k => integProblems[k].label)
    });
    this._gest = createGestureTracker();
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.applyProblem(this.data.config.problem_key);
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#plotCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[simpson] canvas 节点获取失败');
          return;
        }
        this._canvas = res[0].node;
        this._ctx = this._canvas.getContext('2d');
        this.drawPlot();
      });
  },

  onUnload() {
    this._unloaded = true;
    if (this._revealTween) this._revealTween.cancel();
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

  // 加载实验配置（后端不可用时回退默认 sin_pi）
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
      const cfg = data.config || {};
      if (cfg.problem_key && integProblems[cfg.problem_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...cfg }
        });
      }
    } catch (err) {
      console.warn('[simpson] 后端不可用，使用默认配置');
    }
    this.applyProblem(this.data.config.problem_key);
  },

  // 换题/初始化统一入口：应用题库默认区间与 n，刷新节点表与画布
  applyProblem(problemKey) {
    const prob = integProblems[problemKey] || integProblems.sin_pi;
    this._prob = prob;
    const idx = PROBLEM_KEYS.indexOf(problemKey);
    this.setData({
      problemIndex: idx >= 0 ? idx : 0,
      problemLabel: prob.label,
      problemDesc: prob.desc,
      fLabel: prob.fLabel
    });
    // 题库默认区间/n（config 里后端也下发了同一组数值）
    this.applyParams(prob.a, prob.b, prob.n);
  },

  // 应用区间与分段数：刷新节点表/公式卡并重绘；返回 true=已应用，false=参数非法
  applyParams(a, b, n) {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    const prob = this._prob;
    if (!isFinite(a) || !isFinite(b) || !(a < b)) {
      wx.showToast({ title: '区间须满足 a < b', icon: 'none' });
      return false;
    }
    n = Math.round(n);
    if (!(n >= 2 && n <= 64) || n % 2 !== 0) {
      wx.showToast({ title: 'n 须为正偶数，本页手算上限 64', icon: 'none' });
      return false;
    }
    const h = (b - a) / n;
    this.setData({
      aApplied: a,
      bApplied: b,
      nApplied: n,
      aText: String(Number(a.toPrecision(8))),
      bText: String(Number(b.toPrecision(8))),
      nText: String(n),
      hLabel: fmtNum(h, 6),
      fALabel: fmtNum(prob.f(a), 6),
      fBLabel: fmtNum(prob.f(b), 6),
      oddCount: n / 2,
      evenCount: n / 2 - 1,
      ...this.buildNodeRows(prob, a, b, n),
      // 换题/换参数后旧答案作废
      oddSumText: '',
      evenSumText: '',
      valueText: ''
    });
    this._view = null; // 换参数后重新自适应视野
    this.startPanelReveal(n / 2);
    return true;
  },

  // 抛物面板逐块揭示：n/2 个面板自左向右长出（时长随面板数封顶 1.6s）
  startPanelReveal(panels) {
    if (this._revealTween) { this._revealTween.cancel(); this._revealTween = null; }
    this._panelReveal = 0;
    const cv = this.data.fullscreen ? this._fullCanvas : this._canvas;
    this._revealTween = tween({
      duration: Math.min(1600, 320 + panels * 90),
      ease: easeLinear,
      canvas: cv,
      onUpdate: (t) => { this._panelReveal = t; this.drawPlot(); },
      onDone: () => {
        this._panelReveal = 1;
        this._revealTween = null;
        this.drawPlot();
      }
    });
    this.drawPlot(); // 立即出首帧（reveal=0）
  },

  // 节点表行：i、x_i、f(x_i) 与权重（端点 ×1、奇下标 ×4、偶下标 ×2）。
  // n>32 时表太长，只显示头 10 行 + 尾 2 行（手算一般也用不到更大 n）
  buildNodeRows(prob, a, b, n) {
    const rows = [];
    for (let i = 0; i <= n; i++) {
      const x = a + (b - a) * i / n;
      rows.push({
        i,
        x: fmtNum(x, 6),
        y: fmtNum(prob.f(x), 6),
        w: i === 0 || i === n ? '端点 ×1' : (i % 2 === 1 ? '奇 ×4' : '偶 ×2')
      });
    }
    if (n <= 32) {
      return { nodeRows: rows, nodeRowsTail: [], nodeTableTruncated: false };
    }
    return { nodeRows: rows.slice(0, 10), nodeRowsTail: rows.slice(-2), nodeTableTruncated: true };
  },

  // ===== 交互 =====

  // 换题（换积分题库）：区间/n/节点表/画布全部重开
  onProblemChange(e) {
    const idx = Number(e.detail.value);
    const key = PROBLEM_KEYS[idx];
    if (!key || !integProblems[key]) return;
    this.setData({ config: { ...this.data.config, problem_key: key } });
    this.applyProblem(key);
  },

  onAInput(e) { this.setData({ aText: e.detail.value }); },
  onBInput(e) { this.setData({ bText: e.detail.value }); },
  onNInput(e) { this.setData({ nText: e.detail.value }); },

  // 应用自定义区间/n：数值合法即重算节点表与公式卡（提交时随 user_config 上报）
  applyParamsInput() {
    const a = parseNum(this.data.aText);
    const b = parseNum(this.data.bText);
    const n = parseIntStrict(this.data.nText);
    if (a === null || b === null) {
      wx.showToast({ title: '区间端点须为数字', icon: 'none' });
      return;
    }
    if (n === null) {
      wx.showToast({ title: 'n 须为整数', icon: 'none' });
      return;
    }
    if (this.applyParams(a, b, n)) {
      wx.showToast({ title: '已按新参数重开节点表', icon: 'success' });
    }
  },

  onOddSumInput(e) { this.setData({ oddSumText: e.detail.value }); },
  onEvenSumInput(e) { this.setData({ evenSumText: e.detail.value }); },
  onValueInput(e) { this.setData({ valueText: e.detail.value }); },

  // 提交读数评分：value 必填；odd_sum/even_sum 选填随 readings 上报作审计
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, valueText, oddSumText, evenSumText, aApplied, bApplied, nApplied, config } = this.data;
    const value = parseFloat(valueText);
    if (valueText === '' || !isFinite(value)) {
      wx.showToast({ title: '请填写你算出的 S_n', icon: 'none' });
      return;
    }
    const reading = { value };
    // 奇/偶和选填：填了才上报（仅审计，不参与打分）
    const odd = parseFloat(oddSumText);
    if (String(oddSumText).trim() !== '') {
      if (!isFinite(odd)) {
        wx.showToast({ title: 'Σ奇 须为数字', icon: 'none' });
        return;
      }
      reading.odd_sum = odd;
    }
    const even = parseFloat(evenSumText);
    if (String(evenSumText).trim() !== '') {
      if (!isFinite(even)) {
        wx.showToast({ title: 'Σ偶 须为数字', icon: 'none' });
        return;
      }
      reading.even_sum = even;
    }

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 17,
          experiment: 'simpson_integration',
          readings: [reading],
          // 携带实际出题参数：换题/换区间/换 n 时后端按此重算辛普森真值（契约 §12）
          user_config: {
            problem_key: config.problem_key,
            a: aApplied,
            b: bApplied,
            n: nApplied
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
      console.error('[simpson] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // x 处的辛普森逼近曲线值：所属三元组（3 节点）上的拉格朗日二次抛物线
  simpsonCurve(x) {
    const { a, b, n } = { a: this.data.aApplied, b: this.data.bApplied, n: this.data.nApplied };
    const h = (b - a) / n;
    let k = Math.floor((x - a) / (2 * h));
    if (k < 0) k = 0;
    if (k > n / 2 - 1) k = n / 2 - 1;
    const xs = [a + 2 * k * h, a + (2 * k + 1) * h, a + (2 * k + 2) * h];
    const ys = xs.map(xi => this._prob.f(xi));
    return lagrangeP(xs, ys, x);
  },

  // 初始视野：题库横轴范围，y 取 f 与辛普森抛物线采样的并集再外扩 10%
  computeView() {
    const prob = this._prob || integProblems.sin_pi;
    const a = this.data.aApplied, b = this.data.bApplied, n = this.data.nApplied;
    const span = b - a;
    const xMin = a - span * 0.12, xMax = b + span * 0.12;
    const N = 200;
    let yMin = Infinity, yMax = -Infinity;
    for (let k = 0; k <= N; k++) {
      const x = xMin + (xMax - xMin) * k / N;
      const yf = prob.f(x);
      if (isFinite(yf)) {
        yMin = Math.min(yMin, yf);
        yMax = Math.max(yMax, yf);
      }
      if (x >= a && x <= b) {
        const ys = this.simpsonCurve(x);
        if (isFinite(ys)) {
          yMin = Math.min(yMin, ys);
          yMax = Math.max(yMax, ys);
        }
      }
    }
    if (!isFinite(yMin) || !isFinite(yMax)) { yMin = -1; yMax = 1; }
    if (yMin === yMax) { yMin -= 0.5; yMax += 0.5; }
    const pad = (yMax - yMin) * 0.1;
    return clampView({ xMin, xMax, yMin: yMin - pad, yMax: yMax + pad });
  },

  // 复位视野（双击或换题时调用）
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
            console.error('[simpson] 全屏 canvas 节点获取失败');
            return;
          }
          this._fullCanvas = res[0].node;
          this._fullCtx = this._fullCanvas.getContext('2d');
          this.drawPlot();
        });
    });
  },

  exitFullscreen() {
    // 全屏期间小画布跳过重绘，收回后补画一帧
    this.setData({ fullscreen: false }, () => this.drawPlot());
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
    // 全屏期间小画布被隐藏，跳过其重绘（拖动/脉冲每帧只画一块画布）
    if (!this.data.fullscreen && this._canvas && this._ctx) {
      this._drawOn(this._canvas, this._ctx, this.data.canvasWidth, this.data.canvasHeight, 'small');
    }
    if (this.data.fullscreen && this._fullCanvas && this._fullCtx) {
      this._drawOn(this._fullCanvas, this._fullCtx, this.data.fullW, this.data.fullCanvasH, 'full');
    }
  },

  // 在指定 canvas 上绘制 f 曲线、辛普森抛物线、积分区域阴影、节点与刻度网格
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;
    const prob = this._prob || integProblems.sin_pi;
    const f = prob.f;
    const a = this.data.aApplied, b = this.data.bApplied, n = this.data.nApplied;

    // backing store 仅在尺寸变化时重设（每帧重赋 width 会清空画布状态且有开销）
    const pr = this.data.pixelRatio || 1;
    const bw = Math.round(w * pr), bh = Math.round(h * pr);
    if (canvas.width !== bw || canvas.height !== bh) {
      canvas.width = bw;
      canvas.height = bh;
    }
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

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
    const clampY = (py) => Math.max(mT - 1000, Math.min(mT + ph + 1000, py));
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

    // 积分区域阴影：辛普森抛物线与 x 轴围成的面积即 S_n 的几何含义。
    // 逐面板揭示（_panelReveal）：阴影只覆盖已长出的 [a, xEnd]
    const reveal = this._panelReveal == null ? 1 : this._panelReveal;
    const xEnd = a + (b - a) * reveal;
    ctx.fillStyle = 'rgba(230, 126, 34, 0.12)';
    ctx.beginPath();
    const SH = Math.max(24, n * 6);
    for (let i = 0; i <= SH; i++) {
      const x = a + (xEnd - a) * i / SH;
      const py = clampY(Y(this.simpsonCurve(x)));
      if (i === 0) ctx.moveTo(X(x), py);
      else ctx.lineTo(X(x), py);
    }
    ctx.lineTo(X(xEnd), clampY(Y(0)));
    ctx.lineTo(X(a), clampY(Y(0)));
    ctx.closePath();
    ctx.fill();

    // f 曲线（蓝实线；离屏过远的 y 夹取，防 canvas 精度问题）
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    const N = 240;
    let started = false;
    for (let i = 0; i <= N; i++) {
      const x = view.xMin + (view.xMax - view.xMin) * i / N;
      const y = f(x);
      if (!isFinite(y)) {
        started = false;
        continue;
      }
      const py = clampY(Y(y));
      if (!started) {
        ctx.moveTo(X(x), py);
        started = true;
      } else {
        ctx.lineTo(X(x), py);
      }
    }
    ctx.stroke();

    // 辛普森抛物线（橙虚线：每两段一条过三节点的二次曲线）
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.beginPath();
    started = false;
    for (let i = 0; i <= N; i++) {
      const x = a + (xEnd - a) * i / N;
      const y = this.simpsonCurve(x);
      if (!isFinite(y)) {
        started = false;
        continue;
      }
      const py = clampY(Y(y));
      if (!started) {
        ctx.moveTo(X(x), py);
        started = true;
      } else {
        ctx.lineTo(X(x), py);
      }
    }
    ctx.stroke();
    ctx.setLineDash([]);

    // 分点散点（橙心白环，位于抛物线与 f 的交点上；揭示未到的节点先不出现）
    const step = (b - a) / n; // 注意：本方法形参 h 是画布高度，步长另命名避免重复声明
    const dotEvery = n <= 32 ? 1 : Math.ceil(n / 32); // 点太密时抽稀
    for (let i = 0; i <= n; i += dotEvery) {
      const x = a + i * step;
      if (X(x) > X(xEnd) + 0.5) continue;
      const py = clampY(Y(f(x)));
      ctx.beginPath();
      ctx.arc(X(x), py, 4.5, 0, Math.PI * 2);
      ctx.fillStyle = '#e67e22';
      ctx.fill();
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = '#fff';
      ctx.stroke();
    }

    // 生长头：当前已揭示边界的琥珀竖线 + 呼吸光点（帧由揭示 tween 驱动）
    if (reveal > 0 && reveal < 1) {
      const yHead = this.simpsonCurve(xEnd);
      if (isFinite(yHead)) {
        const nowMs = Date.now();
        const hx = X(xEnd), hy = clampY(Y(yHead));
        ctx.strokeStyle = 'rgba(243, 156, 18, 0.85)';
        ctx.lineWidth = 1.2;
        ctx.setLineDash([3, 3]);
        ctx.beginPath();
        ctx.moveTo(hx, clampY(Y(0)));
        ctx.lineTo(hx, hy);
        ctx.stroke();
        ctx.setLineDash([]);
        const pulse = 4.5 + 1.4 * Math.sin(nowMs / 90);
        ctx.fillStyle = '#f39c12';
        ctx.beginPath();
        ctx.arc(hx, hy, pulse, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = 'rgba(243,156,18,0.5)';
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.arc(hx, hy, pulse + 3 + 1.5 * (0.5 + 0.5 * Math.sin(nowMs / 140)), 0, Math.PI * 2);
        ctx.stroke();
      }
    }

    // 区间端点 a/b：绿竖线（积分上下限）
    [a, b].forEach((xv, k) => {
      const pX = X(xv);
      if (pX < mL - 4 || pX > mL + pw + 4) return;
      ctx.strokeStyle = '#2ecc71';
      ctx.lineWidth = 1.5;
      ctx.setLineDash([4, 4]);
      ctx.beginPath();
      ctx.moveTo(pX, mT);
      ctx.lineTo(pX, mT + ph);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.fillStyle = '#2ecc71';
      ctx.font = '11px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText(k === 0 ? 'a' : 'b', clampX(pX), mT + 12);
    });

    // 图例（画在画布内，原生 canvas 上的 view 可能被遮挡）
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    let lx = mL + 8;
    const ly = mT + 14;
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('f(x)', lx + 22, ly + 4);
    lx += 56;
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('辛普森抛物线 ×' + (n / 2), lx + 22, ly + 4);

    ctx.textAlign = 'left';
    ctx.font = '12px sans-serif';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.75)';
    ctx.fillText(this.data.fLabel, mL + 4, mT + 32);
    if (which === 'full') {
      ctx.textAlign = 'right';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
      ctx.fillText('双击图像退出全屏', mL + pw, mT + 14);
    }
  }
});
