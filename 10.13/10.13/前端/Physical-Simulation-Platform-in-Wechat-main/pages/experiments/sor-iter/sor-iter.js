// pages/experiments/sor-iter/sor-iter.js - SOR 逐次超松弛迭代法解线性方程组（数值分析 L15）
// 契约（docs/api-contract.md 附·数值分析批次 L5）：
//   config: { system_key, system_desc, A, b, x0, omega, omega_min, omega_max, omega_hint,
//             tolerance, max_iterations, expected_iterations }
//   readings: [{ solution: [x1, ..., xn], iterations, omega }]（solution 必填；omega 缺省回退 user_config/target/1.0）
//   user_config: { system_key, x0, omega }（换题/换初值/换 ω 时后端按同参数模拟 SOR 重算期望迭代数）
// 教学主线：原方程组 → 选松弛因子 ω → 从 x₀ 起高斯-赛德尔式就地迭代
//   （xᵢ⁽ᵏ⁺¹⁾ = (1−ω)·xᵢ + (ω/aᵢᵢ)·(bᵢ − Σ_{j≠i} aᵢⱼ·xⱼ)，j<i 用刚更新的新值）
//   → 观察 ω 对收敛步数的影响（sys6_poisson 上 ω=1 需 61 步，ω≈1.4 仅 19 步）→ 学生填解提交。
// 方程组库与后端 scoring.go 的 linSystems 同步（utils/numlab.js）。
import { request } from '../../../utils/request';
import { linSystems, fmtNum } from '../../../utils/numlab';
import { createStepPlayer } from '../../../utils/anim';

// 矩阵/向量单元展示：消浮点噪声，5 位有效数字（6 未知数列宽紧凑）；过大/过小用指数记法
function fmtCell(v) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  if (Math.abs(v) < 1e-10) return '0';
  const abs = Math.abs(v);
  if (abs >= 1e7 || abs < 0.001) return v.toExponential(3);
  return String(Number(v.toPrecision(5)));
}

// 下标数字（x₁ ~ x₈）
function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

// 上标数字（迭代序号 x⁽ᵏ⁾）
function sup(d) {
  return String(d).replace(/\d/g, c => '⁰¹²³⁴⁵⁶⁷⁸⁹'.charAt(Number(c)));
}

// SOR 页的系统展示顺序与文案（A/b 数值一律取 linSystems，与后端评分同源）
const SOR_KEYS = ['sys6_poisson', 'sys3_diagdom', 'sys4_tridiag', 'hilbert4'];
const SOR_META = {
  sys6_poisson: {
    label: '泊松 6 阶（SOR 主场）',
    desc: '一维泊松方程差分三对角：ω=1 赛德尔 61 步、最优 ω≈1.4 仅 19 步——调 ω 寻优即本关目标'
  },
  sys3_diagdom: {
    label: '对角占优 3 阶',
    desc: '严格对角占优，SOR 必收敛；解恰为整数 (1, 2, 3)'
  },
  sys4_tridiag: {
    label: '三对角 4 阶（占优）',
    desc: '弹簧链/热传导差分模型，对角占优，SOR 收敛'
  },
  hilbert4: {
    label: 'Hilbert 病态 4 阶（示警）',
    desc: '不对角占优，SOR 迭代发散——观察现象用'
  }
};

// 收敛图归档曲线的短名（图例空间有限；SOR 的对比维度主要是 ω）
const SYSTEM_SHORT = {
  sys6_poisson: '泊松6',
  sys3_diagdom: '占优3',
  sys4_tridiag: '三对角4',
  hilbert4: 'Hilbert4'
};

const DEFAULT_CONFIG = {
  system_key: 'sys6_poisson',
  system_desc: '一维泊松方程差分三对角：ω=1 赛德尔 61 步、最优 ω≈1.4 仅 19 步——调 ω 寻优即本关目标',
  A: [
    [2, -1, 0, 0, 0, 0],
    [-1, 2, -1, 0, 0, 0],
    [0, -1, 2, -1, 0, 0],
    [0, 0, -1, 2, -1, 0],
    [0, 0, 0, -1, 2, -1],
    [0, 0, 0, 0, -1, 2]
  ],
  b: [1, 0, 0, 0, 0, 1],
  x0: [0, 0, 0, 0, 0, 0],
  omega: 1.0,
  omega_min: 0.5,
  omega_max: 1.95,
  omega_hint: 'omega=1 即高斯-赛德尔；试试 1.3~1.5，观察 61 步被加速到约 19 步',
  tolerance: 0.000001,
  max_iterations: 100,
  expected_iterations: 61
};

Page({
  data: {
    levelId: null,
    experimentId: null,
    isPreview: false,
    submitting: false,
    // 自动分拍演示进行中（按钮在「自动演示 / 停止演示」间切换）
    autoPlaying: false,
    // 出题参数（后端 GET /experiments/:levelId 下发，缺省走库内默认）
    config: { ...DEFAULT_CONFIG },
    systemKeys: SOR_KEYS,
    systemLabels: SOR_KEYS.map(k => SOR_META[k].label),
    systemIndex: 0,
    systemDesc: '',
    // 松弛因子 ω（滑块），改 ω 重开迭代便于公平对比收敛步数
    omega: 1.0,
    omegaText: '1.0（高斯-赛德尔）',
    omegaMin: 0.5,
    omegaMax: 1.95,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    n: 6,
    tolText: '',
    maxIterText: '',
    equations: [],
    // 迭代初值输入（字符串）
    x0Inputs: [],
    // 迭代状态：running / converged / diverged / maxiter
    status: 'running',
    k: 0,
    xText: '',
    deltaText: '',
    stepLines: [],
    historyRows: [],
    headerCells: [],
    finished: false,
    solutionInputs: [],
    solutionText: '',
    iterationsText: '',
    // 收敛可视化（components/conv-plot）：convLive 本题运行，convRuns 已归档的不同 ω 运行
    convLive: null,
    convRuns: [],
    // n=3 时传给组件的三维平面数据 {A,b}（默认泊松 6 阶为 null）
    space3d: null
  },

  // 工作数据（A/b、当前 x、已应用初值与 ω），不进 data
  _A: null,
  _b: null,
  _x: null,
  _x0: null,
  _omega: 1.0,
  _delta: 0,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin'
    });
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.restart();
    }
  },

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
      if (data.config && data.config.A && data.config.b) {
        const key = data.config.system_key || 'sys6_poisson';
        // A/b 一律以库内定义为准（与后端评分真值同源），库外的才用下发值
        const def = linSystems[key];
        this.setData({
          experimentId: data.id,
          config: {
            system_key: key,
            system_desc: SOR_META[key] ? SOR_META[key].desc : (data.config.system_desc || ''),
            A: def ? def.A : data.config.A,
            b: def ? def.b : data.config.b,
            x0: data.config.x0 || [],
            omega: Number(data.config.omega) > 0 ? Number(data.config.omega) : 1.0,
            omega_min: Number(data.config.omega_min) > 0 ? Number(data.config.omega_min) : 0.5,
            omega_max: Number(data.config.omega_max) > 1 ? Number(data.config.omega_max) : 1.95,
            omega_hint: data.config.omega_hint || DEFAULT_CONFIG.omega_hint,
            tolerance: Number(data.config.tolerance) > 0 ? Number(data.config.tolerance) : 0.000001,
            max_iterations: Number(data.config.max_iterations) > 0 ? Number(data.config.max_iterations) : 100,
            expected_iterations: data.config.expected_iterations
          },
          systemIndex: this.data.systemKeys.indexOf(key)
        });
      }
    } catch (err) {
      console.warn('[sor-iter] 后端不可用，使用默认配置');
    }
    this.restart();
  },

  // 换题（picker）：随 user_config 提交 system_key，后端按新方程组重算真值与期望迭代数
  onSystemChange(e) {
    const idx = Number(e.detail.value);
    const key = this.data.systemKeys[idx];
    if (!key || key === this.data.config.system_key) return;
    const def = linSystems[key];
    this.setData({
      config: {
        ...this.data.config,
        system_key: key,
        system_desc: SOR_META[key].desc,
        A: def.A,
        b: def.b,
        // 换题后维数可能变化，初值统一重置为零向量（标准教学起点）
        x0: new Array(def.A.length).fill(0),
        expected_iterations: ''
      },
      systemIndex: idx
    });
    this.restart();
  },

  // ω 滑块：拖动中只刷新读数，松手才应用并重开迭代（同 x₀ 的交互节奏）
  onOmegaChanging(e) {
    const w = Number(e.detail.value);
    this.setData({ omegaText: this.omegaLabel(w) });
  },

  onOmegaChange(e) {
    const w = Number(e.detail.value);
    if (Math.abs(w - this._omega) < 1e-9) return; // 值未变不重开
    this.setData({ omega: w, omegaText: this.omegaLabel(w) });
    this.restart();
  },

  // ω 读数文案：1.0 标注高斯-赛德尔
  omegaLabel(w) {
    const s = String(Number(w.toPrecision(3)));
    if (Math.abs(w - 1) < 1e-9) return `${s}（高斯-赛德尔）`;
    if (w < 1) return `${s}（低松弛）`;
    return `${s}（超松弛）`;
  },

  // 重开：按当前出题参数（含 x₀ 与 ω）重建迭代
  restart() {
    const { config, omega } = this.data;
    const n = config.A.length;
    this._A = config.A.map(row => row.map(Number));
    this._b = config.b.map(Number);
    // 滑块上下限以后端 config 为准（出题方可按方程组收窄 ω 范围）
    const omegaMin = Number(config.omega_min) > 0 ? Number(config.omega_min) : 0.5;
    const omegaMax = Number(config.omega_max) > 1 ? Number(config.omega_max) : 1.95;
    let w = Number(omega) > 0 && Number(omega) < 2 ? Number(omega) : 1.0;
    w = Math.max(omegaMin, Math.min(omegaMax, w)); // 越界（如换题后）夹回合法区间
    this._omega = w;
    // 初值长度与维数不符时回退零向量（后端 target.x0 换题同款策略）
    let x0 = (config.x0 || []).map(Number);
    if (x0.length !== n || x0.some(v => !isFinite(v))) {
      x0 = new Array(n).fill(0);
    }
    this._x0 = x0;
    this.setData({
      n,
      equations: config.A.map((row, i) => this.formatEquation(row, config.b[i])),
      x0Inputs: x0.map(v => String(Number(v.toPrecision(6)))),
      omega: this._omega,
      omegaMin,
      omegaMax,
      omegaText: this.omegaLabel(this._omega),
      headerCells: ['k'].concat(Array.from({ length: n }, (_, i) => `x${sub(i + 1)}`)).concat(['max|Δx|']),
      // 三元方程组：每张方程是空间平面，组件画三平面 + 迭代轨迹
      space3d: n === 3 ? { A: config.A.map(row => row.map(Number)), b: config.b.map(Number) } : null
    });
    this.resetIteration();
  },

  // 清零迭代状态（保留出题参数与已应用初值/ω）
  resetIteration() {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    this.stopAutoRun();
    this._x = this._x0.slice();
    this._delta = 0;
    // 上一个 ω（或换题/换初值）的运行归档，调 ω 寻优时几条曲线直接对比，最多保留 6 条
    let convRuns = this.data.convRuns;
    const old = this._conv;
    if (old && old.deltas.length > 0) {
      const tag = old.state === 'converged' ? ` ${old.deltas.length}步✓`
        : old.state === 'diverged' ? ` ${old.deltas.length}步发散`
        : ` ${old.deltas.length}步`;
      convRuns = convRuns.concat([{
        label: old.name + tag,
        deltas: old.deltas.slice(),
        xs: old.xs.slice()
      }]).slice(-6);
    }
    const sysShort = SYSTEM_SHORT[this.data.config.system_key] || this.data.config.system_key;
    const wTxt = String(Number(this._omega.toPrecision(3)));
    const name = `${sysShort}·ω${wTxt}`;
    this._conv = {
      name,
      state: 'running',
      deltas: [],
      xs: [this._x0.slice()] // xs[k] = 第 k 步后的 x（xs[0] 为初值）
    };
    this.setData({
      status: 'running',
      k: 0,
      xText: `(${this._x.map(v => fmtCell(v)).join(', ')})`,
      deltaText: '—',
      stepLines: [],
      historyRows: [],
      finished: false,
      solutionInputs: [],
      solutionText: '',
      iterationsText: '',
      convRuns,
      convLive: { label: `本次 ω=${wTxt}`, state: 'running', deltas: [], xs: [this._x0.slice()] }
    });
  },

  // 原方程组的可读形式：2x₁ − x₂ = 1
  formatEquation(aRow, bVal) {
    const n = aRow.length;
    const terms = [];
    for (let j = 0; j < n; j++) {
      const c = aRow[j];
      if (Math.abs(c) < 1e-12) continue;
      const abs = Math.abs(c);
      const coef = Math.abs(abs - 1) < 1e-12 ? '' : fmtCell(abs);
      const sign = c < 0 ? '−' : (terms.length ? '+' : '');
      terms.push(`${sign}${coef}x${sub(j + 1)}`);
    }
    const lhs = terms.length ? terms.join(' ') : '0';
    return `${lhs} = ${fmtCell(bVal)}`;
  },

  // 刷新与 config 相关的展示字段
  refreshDisplay() {
    const { config } = this.data;
    this.setData({
      tolText: fmtNum(config.tolerance),
      maxIterText: String(config.max_iterations),
      systemDesc: config.system_desc
    });
  },

  // 编辑迭代初值
  onX0Input(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`x0Inputs[${idx}]`]: e.detail.value });
  },

  // 应用初值输入框中的 x₀（重开迭代；提交时随 user_config 上送）
  applyX0() {
    const vals = this.data.x0Inputs.map(s => Number(s));
    if (vals.length !== this.data.n || vals.some(v => !isFinite(v))) {
      wx.showToast({ title: 'x₀ 各分量须为数值', icon: 'none' });
      return;
    }
    this._x0 = vals;
    this.setData({ config: { ...this.data.config, x0: vals } });
    this.resetIteration();
  },

  // 单步迭代（自动播放中交由播放器推进，避免交错重入）
  onStep() {
    if (this._player || this.data.status !== 'running') return;
    this.sorStep();
    this.refreshIteration();
  },

  // 自动演示 / 停止演示（同一按钮切换；分拍播放，每一拍都能看到 δ 曲线与 3D 轨迹推进）
  onAutoRun() {
    if (this._player) {
      this.stopAutoRun();
      return;
    }
    if (this.data.status !== 'running') return;
    this.setData({ autoPlaying: true });
    this._player = createStepPlayer({
      interval: 300,
      maxSteps: (Number(this.data.config.max_iterations) || 100) + 2,
      isDone: () => this.data.status !== 'running',
      step: () => {
        this.sorStep();
        this.refreshIteration();
        return true;
      },
      onEnd: () => {
        this._player = null;
        if (this.data.autoPlaying) this.setData({ autoPlaying: false });
      }
    });
  },

  // 停止分拍演示（重开/换题/卸载时也会调用）
  stopAutoRun() {
    const p = this._player;
    this._player = null;
    if (p) p.cancel();
    if (this.data.autoPlaying) this.setData({ autoPlaying: false });
  },

  onUnload() {
    this._unloaded = true;
    this.stopAutoRun();
  },

  // SOR 一步：xᵢ⁽ᵏ⁺¹⁾ = (1−ω)·xᵢ + (ω/aᵢᵢ)·(bᵢ − Σ_{j≠i} aᵢⱼ·xⱼ)
  // j<i 用刚更新的新值、j>i 用旧值（高斯-赛德尔式就地更新，与后端 sorSolve 一致）
  sorStep() {
    const A = this._A;
    const b = this._b;
    const n = this.data.n;
    const x = this._x;
    const w = this._omega;
    const k = this.data.k;
    // xn 就地逐分量更新：未更新位保持旧值，已更新位为新值
    const xn = x.slice();
    const lines = [];
    for (let i = 0; i < n; i++) {
      let s = b[i];
      const parts = [];
      for (let j = 0; j < n; j++) {
        if (j === i) continue;
        if (Math.abs(A[i][j]) >= 1e-12) {
          parts.push(`${fmtCell(A[i][j])}·${fmtCell(xn[j])}`);
          s -= A[i][j] * xn[j];
        }
      }
      const diag = A[i][i];
      const gs = diag === 0 ? NaN : s / diag;
      xn[i] = (1 - w) * xn[i] + w * gs;
      const rhs = parts.length ? `(${fmtCell(b[i])} − (${parts.join(' + ')}))` : `${fmtCell(b[i])}`;
      lines.push(`x${sub(i + 1)}${sup(`(${k + 1})`)} = ${fmtCell(1 - w)}·${fmtCell(x[i])} + ${fmtCell(w)}·${rhs} / ${fmtCell(diag)} = ${fmtCell(xn[i])}`);
    }
    // 停止准则：max_i |x⁽ᵏ⁺¹⁾ − x⁽ᵏ⁾|（与后端 sorSolve 一致，整轮扫完比较）
    let delta = 0;
    for (let i = 0; i < n; i++) {
      const d = Math.abs(xn[i] - x[i]);
      if (d > delta) delta = d;
    }
    this._x = xn;
    this._delta = delta;

    // 状态推进：发散（非有限/量级爆炸）/ 收敛 / 达到最大步数
    const blown = xn.some(v => !isFinite(v) || Math.abs(v) > 1e12) || !isFinite(delta);
    let status = 'running';
    if (blown) {
      status = 'diverged';
    } else if (delta < this.data.config.tolerance) {
      status = 'converged';
    } else if (k + 1 >= this.data.config.max_iterations) {
      status = 'maxiter';
    }

    // 同步收敛可视化数据（δ 原始值用于对数曲线，xs 用于分量轨迹）
    this._conv.deltas.push(delta);
    this._conv.xs.push(xn.slice());
    this._conv.state = status;

    this.setData({
      k: k + 1,
      status,
      stepLines: lines,
      historyRows: this.data.historyRows.concat([
        {
          k: k + 1,
          cells: xn.map((v, i) => ({ k: i, t: fmtCell(v) })),
          delta: fmtCell(delta)
        }
      ]),
      convLive: {
        label: `本次 ω${String(Number(w.toPrecision(3)))}`,
        state: status,
        deltas: this._conv.deltas.slice(),
        xs: this._conv.xs.slice()
      }
    });
  },

  // 迭代状态展示刷新（收敛时生成解向量输入框）
  refreshIteration() {
    const { status, k, n } = this.data;
    const patch = {
      xText: `(${this._x.map(v => fmtCell(v)).join(', ')})`,
      deltaText: fmtCell(this._delta)
    };
    if (status === 'converged' && !this.data.solutionInputs.length) {
      const inputs = [];
      for (let d = 0; d < n; d++) {
        inputs.push({ label: `x${sub(d + 1)}`, value: '' });
      }
      patch.finished = true;
      patch.solutionInputs = inputs;
      patch.solutionText = `x = (${this._x.map(v => fmtCell(v)).join(', ')})`;
      patch.iterationsText = String(k);
    }
    this.setData(patch);
    this.refreshDisplay();
  },

  // 学生填写解向量
  onSolutionInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`solutionInputs[${idx}].value`]: e.detail.value });
  },

  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, finished, solutionInputs, k } = this.data;
    if (!finished || !solutionInputs.length) {
      wx.showToast({ title: '请先迭代至收敛', icon: 'none' });
      return;
    }
    if (solutionInputs.some(it => it.value === '' || !isFinite(Number(it.value)))) {
      wx.showToast({ title: '请完整填写全部解分量（数值）', icon: 'none' });
      return;
    }
    const solution = solutionInputs.map(it => Number(it.value));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 15,
          experiment: 'sor_iter',
          readings: [{ solution, iterations: k, omega: this._omega }],
          // 携带实际出题参数：换题/换初值/换 ω 时后端按此重算真值与期望迭代数（契约 §12）
          user_config: {
            system_key: this.data.config.system_key,
            x0: this._x0,
            omega: this._omega
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
      console.error('[sor-iter] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
