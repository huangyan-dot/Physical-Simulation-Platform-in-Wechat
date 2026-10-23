// pages/experiments/adams-pc-ode/adams-pc-ode.js - 阿当姆斯预报-校正法（数值分析 L23）
// 契约（docs/api-contract.md 附·数值分析批次 L13）：
//   config: { problem_key, problem_desc, func_desc, a, b, y0, n, method_hint }
//   readings: [{ value, y_col? }]（value=学生递推出的终值 y_N，必填；
//             y_col=逐点列 y_0..y_N，长度 n+1，选填仅记入审计不参与打分）
//   user_config: { problem_key, a, b, n }（换题/换区间/换步数；n 须 4~1024——
//             3 步 RK4 表头 + 至少 1 步阿当姆斯，后端按提交参数重算真值）
// 教学设计：线性多步法 AB4 外推预报 + AM4 内插校正一次（PECE），表头 y_1..y_3
// 用 RK4 启动（与 L22 同式）。页面画斜率场与精确解参考线，每步给出公式与 x
// 代入，f 求值、加权平均留给学生手算填终值；真值是「方法的输出」y_N 而非精确
// 解——四阶法收敛快，抄精确解 e 约 93 分、抄 L22 默认答案 65/24 约 72 分均过线
// 但非满分（过程真伪由 y_col 审计暴露）；抄 L21 答案 ~49 分、抄 L20 答案
// ~30 分均不过线；stiff_decay 多步法绝对稳定区间远小于 RK4，抄精确解
// e^{-10} 直接 0 分。填全 y 列后折线会画上斜率场，递推错误一眼可见。
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { odeProblems, fmtNum, parseNum, parseIntStrict } from '../../../utils/numlab';
import {
  panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';
import { drawSlopeField } from '../../../utils/fieldchart';

const PROBLEM_KEYS = Object.keys(odeProblems);

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      problem_key: 'exp_growth',
      a: 0,
      b: 1,
      y0: 1,
      n: 4,
      method_hint: '阿当姆斯多步法：前 3 步 RK4 启动做表头，第 4 步起 AB4 外推预报 + AM4 内插校正一次（PECE），全局截断误差 O(h⁴)；n 须 ≥ 4（3 步表头+至少 1 步阿当姆斯）'
    },
    problemLabels: [],
    problemIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    problemLabel: '',
    problemDesc: '',
    fLabel: '',
    // 生效参数（input 文本 + 已应用的数值）
    aText: '',
    bText: '',
    nText: '',
    aApplied: 0,
    bApplied: 1,
    nApplied: 4,
    y0Label: '1',
    hLabel: '0.25',
    // 递推分步卡（y_0..y_N：表头 3 步 RK4 四探 + 阿当姆斯 AB4-AM4 预报校正；带选填审计输入）
    stepRows: [],
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
      problemLabels: PROBLEM_KEYS.map(k => odeProblems[k].label)
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
          console.error('[adams-pc-ode] canvas 节点获取失败');
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

  // 加载实验配置（后端不可用时回退默认 exp_growth）
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
      if (cfg.problem_key && odeProblems[cfg.problem_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...cfg }
        });
      }
    } catch (err) {
      console.warn('[adams-pc-ode] 后端不可用，使用默认配置');
    }
    this.applyProblem(this.data.config.problem_key);
  },

  // 换题/初始化统一入口：应用题库默认区间与步数，刷新表格与画布
  applyProblem(problemKey) {
    const prob = odeProblems[problemKey] || odeProblems.exp_growth;
    this._prob = prob;
    const idx = PROBLEM_KEYS.indexOf(problemKey);
    this.setData({
      problemIndex: idx >= 0 ? idx : 0,
      problemLabel: prob.label,
      problemDesc: prob.desc,
      fLabel: prob.fLabel
    });
    // 题库默认区间（n 由 config 带入，后端下发了同一组数值；换题回落库内默认 4）
    this.applyParams(prob.a, prob.b, this.data.config.n || prob.n);
  },

  // 应用区间与步数：刷新递推分步表并重绘；返回 true=已应用，false=参数非法
  applyParams(a, b, n) {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    if (!isFinite(a) || !isFinite(b) || !(a < b)) {
      wx.showToast({ title: '区间须满足 a < b', icon: 'none' });
      return false;
    }
    n = Math.round(n);
    if (!(n >= 4 && n <= 32)) {
      wx.showToast({ title: 'n≥4（3 步 RK4 表头），本页手算上限 32', icon: 'none' });
      return false;
    }
    const prob = this._prob || odeProblems.exp_growth;
    this.setData({
      aApplied: a,
      bApplied: b,
      nApplied: n,
      aText: String(Number(a.toPrecision(8))),
      bText: String(Number(b.toPrecision(8))),
      nText: String(n),
      y0Label: fmtNum(prob.y0, 6),
      hLabel: fmtNum((b - a) / n, 6),
      stepRows: this.buildStepRows(prob, a, b, n),
      // 换题/换参数后旧答案作废
      valueText: ''
    });
    this._view = null; // 换参数后重新自适应视野
    this.drawPlot();
    return true;
  },

  // 递推分步卡：前 3 步 RK4 表头（K1..K4 四探 + 加权平均），
  // 第 4 步起 AB4 预报 + AM4 校正（f 求值与乘加留学生手算）；
  // 每行带选填输入（= y_col 审计，全填或全空），填全后折线画上斜率场
  buildStepRows(prob, a, b, n) {
    const h = (b - a) / n;
    const rows = [{
      key: 'Y0',
      label: 'y₀',
      formula: '题给初值 y(a)',
      substituted: `y₀ = ${fmtNum(prob.y0, 6)}（起点 (a, y₀)，已预填）`,
      hasInput: true,
      input: String(Number(prob.y0.toPrecision(8)))
    }];
    for (let i = 1; i <= n; i++) {
      const sp = this._sub(i - 1), si = this._sub(i);
      if (i <= 3) {
        // 表头：RK4 启动（与 L22 同式，表头同阶精度）
        const xp = a + (i - 1) * h;
        const xm = xp + h / 2;
        const xf = a + i * h;
        rows.push({
          key: 'Y' + i,
          label: `y${si}`,
          stage: '表头 · RK4 启动',
          formula: `K₁ = f(x${sp}, y${sp})`,
          substituted: `x${sp} = ${fmtNum(xp, 6)}，h = ${fmtNum(h, 6)}`,
          formula2: `K₂ = f(x${sp}+h/2, y${sp}+h/2·K₁)`,
          substituted2: `x${sp}+h/2 = ${fmtNum(xm, 6)}`,
          formula3: `K₃ = f(x${sp}+h/2, y${sp}+h/2·K₂)`,
          substituted3: `x${sp}+h/2 = ${fmtNum(xm, 6)}`,
          formula4: `K₄ = f(x${si}, y${sp}+h·K₃)`,
          substituted4: `x${si} = ${fmtNum(xf, 6)}`,
          formula5: `y${si} = y${sp} + h/6·(K₁+2K₂+2K₃+K₄)`,
          substituted5: `h/6 = ${fmtNum(h / 6, 6)}`,
          isFinal: i === n,
          hasInput: true,
          input: ''
        });
      } else {
        // 阿当姆斯：AB4 外推预报 + AM4 内插校正一次（PECE）
        const s1 = this._sub(i - 1), s2 = this._sub(i - 2), s3 = this._sub(i - 3), s4 = this._sub(i - 4);
        const x1 = a + (i - 1) * h, x2 = a + (i - 2) * h;
        const x3 = a + (i - 3) * h, x4 = a + (i - 4) * h, xi = a + i * h;
        rows.push({
          key: 'Y' + i,
          label: `y${si}`,
          stage: 'AB4 预报 → AM4 校正',
          formula: `y${si}ᵖ = y${s1} + h/24·(55f(x${s1},y${s1}) − 59f(x${s2},y${s2}) + 37f(x${s3},y${s3}) − 9f(x${s4},y${s4}))`,
          substituted: `h/24 = ${fmtNum(h / 24, 6)}，x${s1} = ${fmtNum(x1, 6)}，x${s2} = ${fmtNum(x2, 6)}，x${s3} = ${fmtNum(x3, 6)}，x${s4} = ${fmtNum(x4, 6)}`,
          formula2: `y${si} = y${s1} + h/24·(9f(x${si}, y${si}ᵖ) + 19f(x${s1},y${s1}) − 5f(x${s2},y${s2}) + f(x${s3},y${s3}))`,
          substituted2: `x${si} = ${fmtNum(xi, 6)}（只校一次）`,
          isFinal: i === n,
          hasInput: true,
          input: ''
        });
      }
    }
    return rows;
  },

  // 下标数字转 Unicode（仅展示用）
  _sub(n) {
    const map = ['₀', '₁', '₂', '₃', '₄', '₅', '₆', '₇', '₈', '₉'];
    return String(n).split('').map(d => map[Number(d)]).join('');
  },

  // ===== 交互 =====

  // 换题（换 ODE 题库）：区间/步数/表/画布全部重开（换题后 n 回落库内默认 4）
  onProblemChange(e) {
    const idx = Number(e.detail.value);
    const key = PROBLEM_KEYS[idx];
    if (!key || !odeProblems[key]) return;
    this.setData({ config: { ...this.data.config, problem_key: key, n: odeProblems[key].n } });
    this.applyProblem(key);
  },

  onAInput(e) { this.setData({ aText: e.detail.value }); },
  onBInput(e) { this.setData({ bText: e.detail.value }); },
  onNInput(e) { this.setData({ nText: e.detail.value }); },

  // 应用自定义区间/n：数值合法即重算分步表（提交时随 user_config 上报）
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
      wx.showToast({ title: '已按新参数重开表格', icon: 'success' });
    }
  },

  // y 列审计输入（选填，全填或全空才上报）；填全会触发折线重绘
  onYColInput(e) {
    const idx = Number(e.currentTarget.dataset.index);
    this.setData({ [`stepRows[${idx}].input`]: e.detail.value });
    this.drawPlot();
  },

  onValueInput(e) { this.setData({ valueText: e.detail.value }); },

  // 提交读数评分：value 必填；y_col 选填随 readings 上报作审计
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, valueText, stepRows, aApplied, bApplied, nApplied, config } = this.data;
    const value = parseFloat(valueText);
    if (valueText === '' || !isFinite(value)) {
      wx.showToast({ title: '请填写你递推出的终值 y_N', icon: 'none' });
      return;
    }
    const reading = { value };
    // y_col 选填：y₀ 是题给初值（已预填），学生只需填 y₁..y_N；
    // 学生行全填或全空才上报（仅审计，不参与打分）
    const y0Text = String(stepRows[0].input).trim();
    const y0Val = parseFloat(y0Text);
    if (y0Text === '' || !isFinite(y0Val)) {
      wx.showToast({ title: 'y₀（题给初值）须为数字', icon: 'none' });
      return;
    }
    const stuRows = stepRows.slice(1);
    const anyFilled = stuRows.some(r => String(r.input).trim() !== '');
    if (anyFilled) {
      if (stuRows.some(r => {
        const t = String(r.input).trim();
        return t !== '' && !isFinite(parseFloat(t));
      })) {
        wx.showToast({ title: 'y 列须为数字', icon: 'none' });
        return;
      }
      if (stuRows.some(r => String(r.input).trim() === '')) {
        wx.showToast({ title: 'y₁ 起的 y 列要么全填要么全空', icon: 'none' });
        return;
      }
      reading.y_col = [y0Val].concat(stuRows.map(r => parseFloat(r.input)));
    }

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 23,
          experiment: 'adams_pc_ode',
          readings: [reading],
          // 携带实际出题参数：换题/换区间/换步数时后端按此重算 AB4-AM4 真值（契约 §12）
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
      console.error('[adams-pc-ode] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // 学生折线：y 列全填且全为数字时返回数值数组（长度 n+1），否则 null
  studentYs() {
    const rows = this.data.stepRows || [];
    const ys = [];
    for (const r of rows) {
      const t = String(r.input == null ? '' : r.input).trim();
      if (t === '') return null;
      const v = parseFloat(t);
      if (!isFinite(v)) return null;
      ys.push(v);
    }
    return ys;
  },

  // 初始视野：题库横轴范围，y 取精确解与学生折线（已填全时）采样的并集再外扩 10%
  computeView() {
    const prob = this._prob || odeProblems.exp_growth;
    const a = this.data.aApplied, b = this.data.bApplied;
    const span = b - a;
    const xMin = a - span * 0.12, xMax = b + span * 0.12;
    const N = 200;
    let yMin = Infinity, yMax = -Infinity;
    for (let k = 0; k <= N; k++) {
      const x = xMin + (xMax - xMin) * k / N;
      const ye = prob.exact(x);
      if (isFinite(ye)) {
        yMin = Math.min(yMin, ye);
        yMax = Math.max(yMax, ye);
      }
    }
    const sy = this.studentYs();
    if (sy) {
      for (const y of sy) {
        if (isFinite(y)) {
          yMin = Math.min(yMin, y);
          yMax = Math.max(yMax, y);
        }
      }
    }
    if (isFinite(prob.y0)) {
      yMin = Math.min(yMin, prob.y0);
      yMax = Math.max(yMax, prob.y0);
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
            console.error('[adams-pc-ode] 全屏 canvas 节点获取失败');
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

  // 在指定 canvas 上绘制：斜率场、精确解参考线、区间端点 a/b、
  // 初值点 y₀、学生折线（y 列填全时）与刻度网格
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;
    const prob = this._prob || odeProblems.exp_growth;
    const f = prob.f;
    const a = this.data.aApplied, b = this.data.bApplied;
    const n = this.data.nApplied;

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

    // 斜率场（公共绘制）：蓝→琥珀→红编码斜率大小，带箭头，零斜率画空心圆
    drawSlopeField({ ctx, X, Y, view, pw, ph, f });

    // 精确解参考线（绿虚线；仅供对照截断误差，不参与评分）
    ctx.strokeStyle = '#2ecc71';
    ctx.lineWidth = 2;
    ctx.setLineDash([7, 5]);
    ctx.beginPath();
    const N = 240;
    let started = false;
    for (let i = 0; i <= N; i++) {
      const x = view.xMin + (view.xMax - view.xMin) * i / N;
      const y = prob.exact(x);
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

    // 区间端点 a/b：绿竖线（积分区间上下限）
    [a, b].forEach((xv, idx) => {
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
      ctx.fillText(idx === 0 ? 'a' : 'b', clampX(pX), mT + 12);
    });

    // 学生折线（橙实线 + 节点白环 + 终点高亮）：y 列全填时才画，
    // 沿斜率场逐格推进，递推错误（折线脱离斜率场）一眼可见
    const sy = this.studentYs();
    if (sy) {
      const h = (b - a) / n;
      ctx.strokeStyle = '#e67e22';
      ctx.lineWidth = 2.5;
      ctx.setLineDash([]);
      ctx.beginPath();
      for (let i = 0; i <= n; i++) {
        const px = X(a + i * h);
        const py = clampY(Y(sy[i]));
        if (i === 0) ctx.moveTo(px, py);
        else ctx.lineTo(px, py);
      }
      ctx.stroke();
      for (let i = 0; i <= n; i++) {
        const px = X(a + i * h);
        const py = clampY(Y(sy[i]));
        ctx.beginPath();
        ctx.arc(px, py, 4, 0, Math.PI * 2);
        ctx.fillStyle = '#e67e22';
        ctx.fill();
        ctx.lineWidth = 1.5;
        ctx.strokeStyle = '#fff';
        ctx.stroke();
      }
      // 预报-校正可视化：显式四步阿当姆斯先给出预报值 P_{n+1}（红空心点），
      // 校正公式再把它拉到你的橙色节点；红虚线长度即该步的校正量（隐式修正幅度）。
      // 由你已填的列计算，仅作自检；n>20 过密不画。
      if (n <= 20) {
        let labeled = false;
        for (let s = 3; s < n; s++) {
          const win = [sy[s], sy[s - 1], sy[s - 2], sy[s - 3]];
          if (win.some(v => !isFinite(v))) continue;
          const fs = win.map((yy, k) => f(a + (s - k) * h, yy));
          if (fs.some(v => !isFinite(v))) continue;
          const yp = sy[s] + h / 24 * (55 * fs[0] - 59 * fs[1] + 37 * fs[2] - 9 * fs[3]);
          if (!isFinite(yp)) continue;
          const pxp = X(a + (s + 1) * h);
          const rawP = Y(yp), rawC = Y(sy[s + 1]);
          if (pxp < mL - 6 || pxp > mL + pw + 6) continue;
          if (rawP < mT - 40 || rawP > mT + ph + 40 || rawC < mT - 40 || rawC > mT + ph + 40) continue;
          ctx.strokeStyle = 'rgba(231, 76, 60, 0.55)';
          ctx.lineWidth = 1;
          ctx.setLineDash([2, 2]);
          ctx.beginPath();
          ctx.moveTo(pxp, clampY(rawP));
          ctx.lineTo(pxp, clampY(rawC));
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.beginPath();
          ctx.arc(pxp, clampY(rawP), 3.2, 0, Math.PI * 2);
          ctx.strokeStyle = '#e74c3c';
          ctx.lineWidth = 1.3;
          ctx.stroke();
          if (!labeled) { // 首个预报点给文字标注，其余靠图例辨识
            ctx.fillStyle = '#e74c3c';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'left';
            ctx.fillText('预报 P', pxp + 5, clampY(rawP) - 5);
            labeled = true;
          }
        }
        // 左下小图例
        const legY = mT + ph - 9;
        ctx.beginPath();
        ctx.arc(mL + 11, legY, 3.2, 0, Math.PI * 2);
        ctx.strokeStyle = '#e74c3c';
        ctx.lineWidth = 1.3;
        ctx.stroke();
        ctx.fillStyle = 'rgba(255,255,255,0.75)';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'left';
        ctx.fillText('预报点 P，红虚线=校正量', mL + 19, legY + 3);
      }
      // 终点 y_N 高亮环：折线终点即提交值
      ctx.beginPath();
      ctx.arc(X(b), clampY(Y(sy[n])), 8.5, 0, Math.PI * 2);
      ctx.strokeStyle = '#e67e22';
      ctx.lineWidth = 2;
      ctx.setLineDash([]);
      ctx.stroke();
    }

    // 初值点 y₀（橙心白环 + 标注）：折线的固定起点
    const y0 = prob.y0;
    const p0y = clampY(Y(y0));
    ctx.beginPath();
    ctx.arc(X(a), p0y, 5, 0, Math.PI * 2);
    ctx.fillStyle = '#e67e22';
    ctx.fill();
    ctx.lineWidth = 1.5;
    ctx.strokeStyle = '#fff';
    ctx.stroke();
    ctx.fillStyle = 'rgba(230, 126, 34, 0.95)';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('y₀', clampX(X(a) + 2), p0y - 10);

    // 图例（画在画布内，原生 canvas 上的 view 可能被遮挡）
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    let lx = mL + 8;
    const ly = mT + 14;
    ctx.strokeStyle = 'rgba(52, 152, 219, 0.5)';
    ctx.lineWidth = 2;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('f 斜率场', lx + 22, ly + 4);
    lx += 76;
    ctx.strokeStyle = '#2ecc71';
    ctx.lineWidth = 2;
    ctx.setLineDash([7, 5]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('精确解(参考)', lx + 22, ly + 4);
    lx += 108;
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('你的折线', lx + 22, ly + 4);

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
