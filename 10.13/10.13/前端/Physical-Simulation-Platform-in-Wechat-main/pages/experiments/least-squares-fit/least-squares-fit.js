// pages/experiments/least-squares-fit/least-squares-fit.js - 最小二乘拟合（数值分析 L24）
// 契约（docs/api-contract.md 附·数值分析批次 L14）：
//   config: { problem_key, problem_desc, func_desc, m, x_eval, method_hint }
//   readings: [{ value, coef? }]（value=拟合多项式在 x_eval 处的值 P(x_eval)，
//             exp_lnw 为回原域的 ŷ(x_eval)=e^{A+B·x_eval}，必填；
//             coef=学生解法方程得的系数 [c_0..c_m]，长度须 m+1，选填仅记审计不参与打分）
//   user_config: { problem_key, m, x_eval }（换题/换拟合次数/换求值点；
//             m 须 0~min(5, 点数-2)——上界防拟合退化成插值，页面 UI 限 0~2，
//             后端 API 全量收 0~5；自定义数据点 pts_x/pts_y 走 API，页面提供三题库）
// 教学设计：求和表把各列乘积算好、Σ 求和留给学生；法方程给结构（n 已代入），
// Σ 数值与高斯消去解系数都留给学生手算；真值是「方法的输出」P(x_eval) 而非
// 任何数据点值——拟合不过点，抄观测值约 29/30/44 分、抄均值 16 分均不过线；
// quad 题抄真函数 x²=4 恰 58 分差线 2 分、次数用错 1 分；exp 题不线性化直接
// 拟 y 约 12 分不过线；抄三次插值 P3(3.5)=4.4375 约 69 分过线但非满分
// （插值≠拟合，过程真伪由 coef 审计暴露）。画布上散点为观测数据、
// 橙实线为拟合曲线（参考对照，页面现算同款法方程），x_eval 处标拟合点。
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { fitProblems, lsqFit, lsqPolyEval, fmtNum, parseNum } from '../../../utils/numlab';
import {
  panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';

const PROBLEM_KEYS = Object.keys(fitProblems);

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      problem_key: 'lin_line',
      m: 1,
      x_eval: 3.5,
      method_hint: '法方程（幂基 m=1）：n*c0 + (sum x)*c1 = sum y；(sum x)*c0 + (sum x^2)*c1 = sum x*y，高斯消去解 c0..c_m 后求 P(x_eval)；m 须 0~min(5, 点数-2)'
    },
    problemLabels: [],
    problemIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    problemLabel: '',
    problemDesc: '',
    isExp: false,
    // 生效参数（picker/input 文本 + 已应用的数值）
    mOptions: ['0（常数）', '1（直线）', '2（抛物线）'],
    mIndex: 1,
    xEvalText: '',
    mApplied: 1,
    xEvalApplied: 3.5,
    ptsLabel: '4',
    // 求和表（数据点行 × 动态列：x、y[ln 题 Y=ln y]、x²..x^{2m}、x·y..x^m·y；Σ 行留白）
    sumHeaders: [],
    sumRows: [],
    sumTailCells: [],
    // 法方程行（结构给出、n 已代入，Σ 数值与求解留给学生）
    normRows: [],
    // coef 审计输入（c_0..c_m，选填全填或全空）
    coefRows: [],
    xEvalLabel: '',
    // 终值答案输入
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

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      problemLabels: PROBLEM_KEYS.map(k => fitProblems[k].label)
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
          console.error('[least-squares-fit] canvas 节点获取失败');
          return;
        }
        this._canvas = res[0].node;
        this._ctx = this._canvas.getContext('2d');
        this.drawPlot();
      });
  },

  onUnload() {
    this._unloaded = true;
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

  // 加载实验配置（后端不可用时回退默认 lin_line）
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
      if (cfg.problem_key && fitProblems[cfg.problem_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...cfg }
        });
      }
    } catch (err) {
      console.warn('[least-squares-fit] 后端不可用，使用默认配置');
    }
    this.applyProblem(this.data.config.problem_key);
  },

  // 换题/初始化统一入口：应用题库默认 m 与求值点，刷新表格与画布
  applyProblem(problemKey) {
    const prob = fitProblems[problemKey] || fitProblems.lin_line;
    this._prob = prob;
    const idx = PROBLEM_KEYS.indexOf(problemKey);
    this.setData({
      problemIndex: idx >= 0 ? idx : 0,
      problemLabel: prob.label,
      problemDesc: prob.desc,
      isExp: !!prob.ln
    });
    // 题库默认 m / x_eval（config 下发同一组数值；换题回落新题库内默认）
    this.applyParams(this.data.config.m != null ? this.data.config.m : prob.m,
      this.data.config.x_eval != null ? this.data.config.x_eval : prob.xEval);
  },

  // 应用拟合次数与求值点：刷新求和表/法方程/系数输入并重绘；返回 true=已应用，false=参数非法
  applyParams(m, xEval) {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    m = Math.round(m);
    if (!(m >= 0 && m <= 2)) {
      wx.showToast({ title: '页面支持 m=0~2（后端 API 收 0~min(5, 点数-2)）', icon: 'none' });
      return false;
    }
    if (!isFinite(xEval)) {
      wx.showToast({ title: 'x_eval 须为数字', icon: 'none' });
      return false;
    }
    const prob = this._prob || fitProblems.lin_line;
    // m 不得退化成插值：页面点数最多 5（quad），m=2 合法；m 上界已在 picker 限死
    this.setData({
      mApplied: m,
      xEvalApplied: xEval,
      mIndex: m,
      xEvalText: String(Number(xEval.toPrecision(8))),
      ptsLabel: String(prob.xs.length),
      sumHeaders: this.buildSumHeaders(prob, m),
      sumRows: this.buildSumRows(prob, m),
      sumTailCells: Array(this.buildSumHeaders(prob, m).length - 1).fill('?'),
      normRows: this.buildNormRows(prob, m),
      coefRows: Array.from({ length: m + 1 }, (_, i) => ({
        key: 'c' + i,
        label: prob.ln && m === 1 ? (i === 0 ? 'c₀（=A）' : 'c₁（=B）') : `c${this._sub(i)}`
      })),
      xEvalLabel: fmtNum(xEval, 6),
      // 换题/换参数后旧答案作废
      valueText: ''
    });
    this._view = null; // 换参数后重新自适应视野
    this.drawPlot();
    return true;
  },

  // 求和表表头：k | xₖ | yₖ |（ln 题 Yₖ=ln yₖ）| xₖ²..xₖ^{2m} | xₖ·yₖ..xₖ^m·yₖ
  buildSumHeaders(prob, m) {
    const yCol = prob.ln ? 'Yₖ=ln yₖ' : 'yₖ';
    const heads = ['k', 'xₖ', yCol];
    for (let p = 2; p <= 2 * m; p++) heads.push(`xₖ${this._sup(p)}`);
    const yName = prob.ln ? 'Y' : 'y';
    for (let j = 1; j <= m; j++) {
      heads.push(j === 1 ? `xₖ·${yName}ₖ` : `xₖ${this._sup(j)}·${yName}ₖ`);
    }
    return heads;
  },

  // 求和表数据行：每格乘积已算好（同 L17 分点表先例），Σ 行在 WXML 里留白
  buildSumRows(prob, m) {
    const ln = !!prob.ln;
    return prob.xs.map((x, k) => {
      const y = prob.ys[k];
      const Y = ln ? Math.log(y) : y;
      const cells = [String(k + 1), fmtNum(x, 6), fmtNum(ln ? Y : y, 6)];
      for (let p = 2; p <= 2 * m; p++) cells.push(fmtNum(Math.pow(x, p), 6));
      for (let j = 1; j <= m; j++) cells.push(fmtNum(Math.pow(x, j) * Y, 6));
      return { key: 'P' + k, cells };
    });
  },

  // 法方程卡：每行给结构（n=点数已代入，Σ 记号指向求和表的列），
  // Σ 数值、消去求解、代回求 P(x_eval) 全部留给学生
  buildNormRows(prob, m) {
    const n = prob.xs.length;
    const rows = [];
    for (let j = 0; j <= m; j++) {
      const terms = [];
      for (let i = 0; i <= m; i++) {
        const p = i + j;
        const coef = p === 0 ? `${n}` : `(Σx${this._sup(p)})`;
        terms.push(`${coef}·c${this._sub(i)}`);
      }
      const rhs = j === 0 ? 'Σy' : `Σx${this._sup(j)}·y`;
      rows.push({
        key: 'E' + j,
        formula: `${terms.join(' + ')} = ${rhs}`,
        substituted: `法方程第 ${j + 1} 行：n=${n} 已代入，各 Σ 从求和表对应列求和`
      });
    }
    return rows;
  },

  // 下标数字转 Unicode（仅展示用）
  _sub(n) {
    const map = ['₀', '₁', '₂', '₃', '₄', '₅', '₆', '₇', '₈', '₉'];
    return String(n).split('').map(d => map[Number(d)]).join('');
  },

  // 上标数字转 Unicode（仅展示用）
  _sup(n) {
    const map = ['⁰', '¹', '²', '³', '⁴', '⁵', '⁶', '⁷', '⁸', '⁹'];
    return String(n).split('').map(d => map[Number(d)]).join('');
  },

  // ===== 交互 =====

  // 换题（换拟合题库）：m/x_eval/表格/画布全部重开（换题后回落库内默认）
  onProblemChange(e) {
    const idx = Number(e.detail.value);
    const key = PROBLEM_KEYS[idx];
    if (!key || !fitProblems[key]) return;
    this.setData({
      config: {
        ...this.data.config,
        problem_key: key,
        m: fitProblems[key].m,
        x_eval: fitProblems[key].xEval
      }
    });
    this.applyProblem(key);
  },

  // 换拟合次数：保留当前 x_eval 重开表格
  onMChange(e) {
    const m = Number(e.detail.value);
    this.applyParams(m, this.data.xEvalApplied);
  },

  onXEvalInput(e) { this.setData({ xEvalText: e.detail.value }); },

  // 应用自定义求值点：合法即重开（提交时随 user_config 上报）
  applyXEvalInput() {
    const xEval = parseNum(this.data.xEvalText);
    if (xEval === null) {
      wx.showToast({ title: 'x_eval 须为数字', icon: 'none' });
      return;
    }
    if (this.applyParams(this.data.mApplied, xEval)) {
      wx.showToast({ title: '已按新求值点重开表格', icon: 'success' });
    }
  },

  // coef 审计输入（选填，全填或全空才上报）
  onCoefInput(e) {
    const idx = Number(e.currentTarget.dataset.index);
    this.setData({ [`coefRows[${idx}].input`]: e.detail.value });
  },

  onValueInput(e) { this.setData({ valueText: e.detail.value }); },

  // 提交读数评分：value 必填；coef 选填随 readings 上报作审计
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, valueText, coefRows, mApplied, xEvalApplied, config } = this.data;
    const value = parseFloat(valueText);
    if (valueText === '' || !isFinite(value)) {
      wx.showToast({ title: '请填写拟合值 P(x_eval)', icon: 'none' });
      return;
    }
    const reading = { value };
    // coef 选填：全填或全空（长度须 m+1），填了才上报（仅审计，不参与打分）
    const anyFilled = coefRows.some(r => String(r.input || '').trim() !== '');
    if (anyFilled) {
      const coefs = coefRows.map(r => parseFloat(r.input));
      if (coefs.some((v, i) => String(coefRows[i].input || '').trim() !== '' && !isFinite(v))) {
        wx.showToast({ title: '系数须为数字', icon: 'none' });
        return;
      }
      if (coefs.some((v, i) => String(coefRows[i].input || '').trim() === '')) {
        wx.showToast({ title: 'coef 要么全填要么全空', icon: 'none' });
        return;
      }
      reading.coef = coefs;
    }

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 24,
          experiment: 'least_squares_fit',
          readings: [reading],
          // 携带实际出题参数：换题/换 m/换求值点时后端按此重算法方程真值（契约 §12）
          user_config: {
            problem_key: config.problem_key,
            m: mApplied,
            x_eval: xEvalApplied
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
      console.error('[least-squares-fit] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // 页面参考拟合曲线（与后端同式法方程现算；仅供画图对照，不参与评分）
  refCurve() {
    const prob = this._prob || fitProblems.lin_line;
    const m = this.data.mApplied;
    const cs = lsqFit(prob.xs, prob.ys, m, !!prob.ln);
    const ln = !!prob.ln;
    return (x) => {
      const v = lsqPolyEval(cs, x);
      return ln ? Math.exp(v) : v;
    };
  },

  // 初始视野：横轴数据 x 范围，y 取观测点与拟合曲线采样的并集再外扩 10%
  computeView() {
    const prob = this._prob || fitProblems.lin_line;
    const xs = prob.xs;
    const span = Math.max(...xs) - Math.min(...xs);
    const xMin = Math.min(...xs) - span * 0.12;
    const xMax = Math.max(...xs) + span * 0.12;
    const curve = this.refCurve();
    const N = 200;
    let yMin = Infinity, yMax = -Infinity;
    for (const y of prob.ys) {
      yMin = Math.min(yMin, y);
      yMax = Math.max(yMax, y);
    }
    for (let k = 0; k <= N; k++) {
      const x = xMin + (xMax - xMin) * k / N;
      const yc = curve(x);
      if (isFinite(yc)) {
        yMin = Math.min(yMin, yc);
        yMax = Math.max(yMax, yc);
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
            console.error('[least-squares-fit] 全屏 canvas 节点获取失败');
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

  // 在指定 canvas 上绘制：散点、拟合曲线（参考）、x_eval 竖线与拟合点、刻度网格
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;
    const prob = this._prob || fitProblems.lin_line;
    const xEval = this.data.xEvalApplied;
    const curve = this.refCurve();

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

    // 拟合曲线（橙实线，参考对照：页面与后端同式法方程现算，不给数值只给形状）
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    const N = 240;
    let started = false;
    for (let i = 0; i <= N; i++) {
      const x = view.xMin + (view.xMax - view.xMin) * i / N;
      const y = curve(x);
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

    // x_eval 竖线（绿虚线）与拟合点（绿圆环）：提交的 P(x_eval) 位置
    const pX = X(xEval);
    if (pX >= mL - 4 && pX <= mL + pw + 4) {
      ctx.strokeStyle = '#2ecc71';
      ctx.lineWidth = 1.5;
      ctx.setLineDash([4, 4]);
      ctx.beginPath();
      ctx.moveTo(pX, mT);
      ctx.lineTo(pX, mT + ph);
      ctx.stroke();
      ctx.setLineDash([]);
      const fy = curve(xEval);
      if (isFinite(fy)) {
        ctx.beginPath();
        ctx.arc(pX, clampY(Y(fy)), 6, 0, Math.PI * 2);
        ctx.strokeStyle = '#2ecc71';
        ctx.lineWidth = 2;
        ctx.stroke();
      }
      ctx.fillStyle = '#2ecc71';
      ctx.font = '11px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText('x_eval', clampX(pX), mT + 12);
    }

    // 残差竖线（红虚线）：观测点到拟合曲线的纵向距离 rₖ=yₖ−P(xₖ)，
    // 最小二乘所最小化的正是 Σrₖ²——线越短整体拟合越好（画在散点下层）
    ctx.lineWidth = 1.2;
    prob.xs.forEach((xv, k) => {
      const yv = prob.ys[k];
      const yc = curve(xv);
      if (!isFinite(yv) || !isFinite(yc)) return;
      const px = X(xv);
      if (px < mL - 4 || px > mL + pw + 4) return;
      ctx.strokeStyle = 'rgba(231, 76, 60, 0.7)';
      ctx.setLineDash([3, 3]);
      ctx.beginPath();
      ctx.moveTo(px, clampY(Y(yv)));
      ctx.lineTo(px, clampY(Y(yc)));
      ctx.stroke();
    });
    ctx.setLineDash([]);

    // 观测散点（蓝实心白环）：拟合只"靠近"不过点，配合红色残差线一眼可见偏差
    for (let k = 0; k < prob.xs.length; k++) {
      const px = X(prob.xs[k]);
      const py = clampY(Y(prob.ys[k]));
      ctx.beginPath();
      ctx.arc(px, py, 4.5, 0, Math.PI * 2);
      ctx.fillStyle = '#3498db';
      ctx.fill();
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = '#fff';
      ctx.stroke();
    }

    // 图例（画在画布内，原生 canvas 上的 view 可能被遮挡）
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    let lx = mL + 8;
    const ly = mT + 14;
    ctx.beginPath();
    ctx.arc(lx + 9, ly, 4, 0, Math.PI * 2);
    ctx.fillStyle = '#3498db';
    ctx.fill();
    ctx.lineWidth = 1.5;
    ctx.strokeStyle = '#fff';
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('观测点', lx + 18, ly + 4);
    lx += 62;
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('拟合(参考)', lx + 22, ly + 4);
    lx += 88;
    ctx.strokeStyle = '#2ecc71';
    ctx.lineWidth = 1.5;
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('x_eval', lx + 22, ly + 4);
    lx += 62;
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.85)';
    ctx.lineWidth = 1.2;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('残差 rₖ', lx + 22, ly + 4);

    ctx.textAlign = 'left';
    ctx.font = '12px sans-serif';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.75)';
    ctx.fillText(this.data.problemLabel, mL + 4, mT + 32);
    if (which === 'full') {
      ctx.textAlign = 'right';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
      ctx.fillText('双击图像退出全屏', mL + pw, mT + 14);
    }
  }
});
