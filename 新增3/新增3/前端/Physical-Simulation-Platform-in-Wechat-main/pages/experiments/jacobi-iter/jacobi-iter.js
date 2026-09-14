// pages/experiments/jacobi-iter/jacobi-iter.js - 雅可比迭代法解线性方程组（数值分析 L14）
// 契约（docs/api-contract.md 附·数值分析批次 L4）：
//   config: { system_key, system_desc, A, b, x0, tolerance, max_iterations, expected_iterations }
//   readings: [{ solution: [x1, ..., xn], iterations }]（solution 必填；iterations 1~max_iterations）
//   user_config: { system_key, x0 }（换题/换初值时后端按同初值模拟雅可比重算期望迭代数）
// 教学主线：原方程组 → 从 x₀ 起逐步迭代（xᵢ⁽ᵏ⁺¹⁾ = (bᵢ − Σ_{j≠i} aᵢⱼxⱼ⁽ᵏ⁾)/aᵢᵢ）
//   → 观察 max|Δx| 收敛（对角占优）/发散（不占优，教学反例）→ 学生填收敛解提交。
// 方程组库与后端 scoring.go 的 linSystems 同步（utils/numlab.js）。
import { request } from '../../../utils/request';
import { linSystems, fmtNum } from '../../../utils/numlab';

// 矩阵/向量单元展示：消浮点噪声，6 位有效数字；过大/过小用指数记法
function fmtCell(v) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  if (Math.abs(v) < 1e-10) return '0';
  const abs = Math.abs(v);
  if (abs >= 1e7 || abs < 0.001) return v.toExponential(3);
  return String(Number(v.toPrecision(6)));
}

// 下标数字（x₁ ~ x₈）
function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

// 上标数字（迭代序号 x⁽ᵏ⁾）
function sup(d) {
  return String(d).replace(/\d/g, c => '⁰¹²³⁴⁵⁶⁷⁸⁹'.charAt(Number(c)));
}

// 雅可比页的系统展示顺序与文案（A/b 数值一律取 linSystems，与后端评分同源）
const JACOBI_KEYS = ['sys3_diagdom', 'sys4_tridiag', 'sys3_basic', 'hilbert4'];
const JACOBI_META = {
  sys3_diagdom: {
    label: '对角占优 3 阶（推荐）',
    desc: '严格对角占优，雅可比必收敛；x₀ = 零向量时约 14 步收敛到整数解'
  },
  sys4_tridiag: {
    label: '三对角 4 阶（占优）',
    desc: '弹簧链/热传导差分模型，对角占优，雅可比收敛'
  },
  sys3_basic: {
    label: '经典 3 阶（临界）',
    desc: '对角优势临界（不严格占优），雅可比不保证收敛——观察现象用'
  },
  hilbert4: {
    label: 'Hilbert 病态 4 阶（示警）',
    desc: '不对角占优，雅可比迭代发散——与 L13 列主元消去对照'
  }
};

// 收敛图归档曲线的短名（图例空间有限）
const SYSTEM_SHORT = {
  sys3_diagdom: '对角占优3阶',
  sys4_tridiag: '三对角4阶',
  sys3_basic: '经典3阶',
  hilbert4: 'Hilbert4阶'
};

const DEFAULT_CONFIG = {
  system_key: 'sys3_diagdom',
  system_desc: '严格对角占优，雅可比必收敛；x₀ = 零向量时约 14 步收敛到整数解',
  A: [[10, 2, 1], [1, 10, 2], [2, 1, 10]],
  b: [17, 27, 34],
  x0: [0, 0, 0],
  tolerance: 0.000001,
  max_iterations: 100,
  expected_iterations: 14
};

Page({
  data: {
    levelId: null,
    experimentId: null,
    isPreview: false,
    submitting: false,
    // 出题参数（后端 GET /experiments/:levelId 下发，缺省走库内默认）
    config: { ...DEFAULT_CONFIG },
    systemKeys: JACOBI_KEYS,
    systemLabels: JACOBI_KEYS.map(k => JACOBI_META[k].label),
    systemIndex: 0,
    systemDesc: '',
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    n: 3,
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
    // 收敛可视化（components/conv-plot）：convLive 本次运行，convRuns 已归档的历史运行
    convLive: null,
    convRuns: [],
    // n=3 时传给组件的三维平面数据 {A,b}，n≠3 为 null（三维标签自动隐藏）
    space3d: null
  },

  // 工作数据（A/b、当前 x、已应用初值），不进 data
  _A: null,
  _b: null,
  _x: null,
  _x0: null,
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
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      const data = res.data || res || {};
      if (data.config && data.config.A && data.config.b) {
        const key = data.config.system_key || 'sys3_diagdom';
        // A/b 一律以库内定义为准（与后端评分真值同源），库外的才用下发值
        const def = linSystems[key];
        this.setData({
          experimentId: data.id,
          config: {
            system_key: key,
            system_desc: JACOBI_META[key] ? JACOBI_META[key].desc : (data.config.system_desc || ''),
            A: def ? def.A : data.config.A,
            b: def ? def.b : data.config.b,
            x0: data.config.x0 || [],
            tolerance: Number(data.config.tolerance) > 0 ? Number(data.config.tolerance) : 0.000001,
            max_iterations: Number(data.config.max_iterations) > 0 ? Number(data.config.max_iterations) : 100,
            expected_iterations: data.config.expected_iterations
          },
          systemIndex: this.data.systemKeys.indexOf(key)
        });
      }
    } catch (err) {
      console.warn('[jacobi-iter] 后端不可用，使用默认配置');
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
        system_key: key,
        system_desc: JACOBI_META[key].desc,
        A: def.A,
        b: def.b,
        // 换题后维数可能变化，初值统一重置为零向量（标准教学起点）
        x0: new Array(def.A.length).fill(0),
        tolerance: this.data.config.tolerance,
        max_iterations: this.data.config.max_iterations,
        expected_iterations: ''
      },
      systemIndex: idx
    });
    this.restart();
  },

  // 重开：按当前出题参数（含 x0）重建迭代
  restart() {
    const { config } = this.data;
    const n = config.A.length;
    this._A = config.A.map(row => row.map(Number));
    this._b = config.b.map(Number);
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
      headerCells: ['k'].concat(Array.from({ length: n }, (_, i) => `x${sub(i + 1)}`)).concat(['max|Δx|']),
      // 三元方程组：每张方程是空间平面，组件画三平面 + 迭代轨迹
      space3d: n === 3 ? { A: config.A.map(row => row.map(Number)), b: config.b.map(Number) } : null
    });
    this.resetIteration();
  },

  // 清零迭代状态（保留出题参数与已应用初值）
  resetIteration() {
    const n = this.data.n;
    this._x = this._x0.slice();
    this._delta = 0;
    // 上一次已跑出的运行归档（换题/换初值后可在收敛图上对比），最多保留 6 条
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
    const name = SYSTEM_SHORT[this.data.config.system_key] || this.data.config.system_key;
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
      convLive: { label: `本次（${name}）`, deltas: [], xs: [this._x0.slice()] }
    });
  },

  // 原方程组的可读形式：10x₁ + 2x₂ + x₃ = 17
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

  // 单步迭代
  onStep() {
    if (this.data.status !== 'running') return;
    this.jacobiStep();
    this.refreshIteration();
  },

  // 自动演示：迭代至收敛/发散/达到最大步数
  onAutoRun() {
    if (this.data.status !== 'running') return;
    while (this.data.status === 'running' && this.data.k < this.data.config.max_iterations) {
      this.jacobiStep();
    }
    this.refreshIteration();
  },

  // 雅可比一步：xᵢ⁽ᵏ⁺¹⁾ = (bᵢ − Σ_{j≠i} aᵢⱼ·xⱼ⁽ᵏ⁾) / aᵢᵢ（各分量同用上轮整体值）
  jacobiStep() {
    const A = this._A;
    const b = this._b;
    const n = this.data.n;
    const x = this._x;
    const k = this.data.k;
    const xn = new Array(n);
    const lines = [];
    for (let i = 0; i < n; i++) {
      let s = b[i];
      const parts = [];
      for (let j = 0; j < n; j++) {
        if (j === i) continue;
        if (Math.abs(A[i][j]) >= 1e-12) {
          parts.push(`${fmtCell(A[i][j])}·${fmtCell(x[j])}`);
          s -= A[i][j] * x[j];
        }
      }
      const diag = A[i][i];
      xn[i] = diag === 0 ? NaN : s / diag;
      const rhs = parts.length ? `(${fmtCell(b[i])} − (${parts.join(' + ')}))` : `${fmtCell(b[i])}`;
      lines.push(`x${sub(i + 1)}${sup(`(${k + 1})`)} = ${rhs} / ${fmtCell(diag)} = ${fmtCell(xn[i])}`);
    }
    // 停止准则：max_i |x⁽ᵏ⁺¹⁾ − x⁽ᵏ⁾|（与后端 jacobiSolve 一致）
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
        label: `本次（${this._conv.name}）`,
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
    const { levelId, finished, solutionInputs, k, config } = this.data;
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
          level_id: parseInt(levelId, 10) || 14,
          experiment: 'jacobi_iter',
          readings: [{ solution, iterations: k }],
          // 携带实际出题参数：换题/换初值时后端按此重算真值与期望迭代数（契约 §12）
          user_config: {
            system_key: config.system_key,
            x0: this._x0
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
      console.error('[jacobi-iter] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
