// pages/experiments/gauss-elim/gauss-elim.js - 高斯消去法解线性方程组（数值分析 L13）
// 契约（docs/api-contract.md 附·数值分析批次 L3）：
//   config: { system_key, system_desc, A, b, tolerance }
//   readings: [{ solution: [x1, ..., xn] }]（长度须等于未知数个数）
//   user_config: { system_key }（换题时后端按提交方程组重算真值解）
// 教学主线：原方程组 → 增广矩阵 → 逐列选列主元（学生点选，教数值稳定性）
//   → 消元成上三角 → 回代逐步出解 → 学生填解向量提交。
// 方程组库与后端 scoring.go 的 linSystems 同步（utils/numlab.js）。
import { request } from '../../../utils/request';
import { linSystems, fmtNum } from '../../../utils/numlab';

// 矩阵单元展示：消去浮点噪声（如 1e-15 显示为 0），6 位有效数字
function fmtCell(v) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  if (Math.abs(v) < 1e-10) return '0';
  return String(Number(v.toPrecision(6)));
}

// 下标数字（x₁ ~ x₈）
function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

const DEFAULT_CONFIG = {
  system_key: 'sys3_basic',
  system_desc: '良态方程组，解恰为整数，手工消去即可完成',
  A: [[2, 1, -1], [-3, -1, 2], [-2, 1, 2]],
  b: [8, -11, -3],
  tolerance: 0.000001
};

Page({
  data: {
    levelId: null,
    experimentId: null,
    isPreview: false,
    submitting: false,
    // 出题参数（后端 GET /experiments/:levelId 下发，缺省走库内默认）
    config: { ...DEFAULT_CONFIG },
    systemKeys: [],
    systemLabels: [],
    systemIndex: 0,
    systemDesc: '',
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    n: 3,
    tolText: '',
    equations: [],
    matrixRows: [],
    // 消元状态机：elim（逐列选主元）→ back（逐步回代）→ done
    stage: 'elim',
    col: 0,
    candidates: [],
    hint: '',
    history: [],
    backLines: [],
    solutionInputs: [],
    solutionText: '',
    finished: false,
    // 解向量条形图数据（components/sol-bars）：未回代求出的分量为 null
    chartValues: [],
    // n=3 时传给组件的三维平面数据（用原始 A/b，消元后的方程不代表原平面）
    space3d: null
  },

  // 工作矩阵（增广 n x (n+1)）与解向量（回代逐格填充），不进 data
  _M: null,
  _x: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    const keys = Object.keys(linSystems);
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      systemKeys: keys,
      systemLabels: keys.map(k => linSystems[k].label)
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
        const key = data.config.system_key || 'sys3_basic';
        // A/b 一律以库内定义为准（与后端评分真值同源），库外的才用下发值
        const def = linSystems[key];
        this.setData({
          experimentId: data.id,
          config: {
            system_key: key,
            system_desc: data.config.system_desc || (def ? def.desc : ''),
            A: def ? def.A : data.config.A,
            b: def ? def.b : data.config.b,
            tolerance: Number(data.config.tolerance) > 0 ? Number(data.config.tolerance) : 0.000001
          },
          systemIndex: this.data.systemKeys.indexOf(key)
        });
      }
    } catch (err) {
      console.warn('[gauss-elim] 后端不可用，使用默认配置');
    }
    this.restart();
  },

  // 换题（picker）：随 user_config 提交 system_key，后端按新方程组重算真值
  onSystemChange(e) {
    const idx = Number(e.detail.value);
    const key = this.data.systemKeys[idx];
    if (!key || key === this.data.config.system_key) return;
    const def = linSystems[key];
    this.setData({
      config: {
        system_key: key,
        system_desc: def.desc,
        A: def.A,
        b: def.b,
        tolerance: this.data.config.tolerance
      },
      systemIndex: idx
    });
    this.restart();
  },

  // 重开求解：重建增广矩阵；题目展示（原方程组）冻结在初始 A/b 上，不随消元变化
  restart() {
    const { config } = this.data;
    const n = config.A.length;
    const M = config.A.map((row, i) => row.map(Number).concat(Number(config.b[i])));
    this._M = M;
    this._x = null;
    this.setData({
      n,
      equations: config.A.map((row, i) => this.formatEquation(row, config.b[i])),
      stage: 'elim',
      col: 0,
      history: [],
      hint: '',
      backLines: [],
      solutionInputs: [],
      solutionText: '',
      finished: false,
      chartValues: new Array(n).fill(null),
      // 三维平面用原始方程组（行交换不改变平面；消元会换成等价方程，学生认原平面）
      space3d: n === 3 ? { A: config.A.map(row => row.map(Number)), b: config.b.map(Number) } : null
    });
    this.refreshDisplay();
  },

  // 回代进度快照：已求出的分量给数值，其余 null（条形图逐根长出）
  snapshotChartValues() {
    const n = this.data.n;
    const x = this._x || [];
    const arr = new Array(n);
    for (let j = 0; j < n; j++) {
      arr[j] = isFinite(x[j]) ? x[j] : null;
    }
    return arr;
  },

  // 刷新展示字段（增广矩阵、候选主元、容差；原方程组在 restart 冻结）
  refreshDisplay() {
    const { config, n, stage, col } = this.data;
    const M = this._M;
    this.setData({
      tolText: fmtNum(config.tolerance),
      systemDesc: config.system_desc,
      matrixRows: M.map((row, ri) => ({
        k: 'r' + ri,
        cls: stage === 'elim' && ri < col ? 'done-row' : '',
        cells: row.map((v, ci) => ({
          k: 'c' + ci,
          t: fmtCell(v),
          cls: (ci === n ? 'cell-b' : '') + (stage === 'elim' && ci === col && ri >= col ? ' cell-pc' : '')
        }))
      })),
      candidates: stage === 'elim' ? this.buildCandidates() : []
    });
  },

  // 原方程组的可读形式：2x₁ + x₂ − x₃ = 8
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

  // 当前列的候选主元行（含 |a| 与是否列最大）
  buildCandidates() {
    const M = this._M;
    const { n, col } = this.data;
    let maxRow = col;
    for (let i = col + 1; i < n; i++) {
      if (Math.abs(M[i][col]) > Math.abs(M[maxRow][col])) maxRow = i;
    }
    const cands = [];
    for (let i = col; i < n; i++) {
      cands.push({
        row: i,
        absText: fmtCell(Math.abs(M[i][col])),
        isMax: i === maxRow
      });
    }
    return cands;
  },

  // 学生点选主元行 → 执行交换与消元
  onPickPivot(e) {
    if (this.data.stage !== 'elim') return;
    const p = Number(e.currentTarget.dataset.row);
    const k = this.data.col;
    if (Math.abs(this._M[p][k]) < 1e-12) {
      wx.showToast({ title: '该行此列为 0，不能作主元', icon: 'none' });
      return;
    }
    this.eliminateColumn(k, p, true);
    this.afterElimStep();
  },

  // 对第 k 列以第 p 行为主元执行交换 + 消元；manual=true 时对非列主元选择给出提示
  eliminateColumn(k, p, manual) {
    const M = this._M;
    const n = this.data.n;
    let maxRow = k;
    for (let i = k + 1; i < n; i++) {
      if (Math.abs(M[i][k]) > Math.abs(M[maxRow][k])) maxRow = i;
    }
    const isMax = p === maxRow;
    const swapText = p !== k ? `，交换 R${k + 1} ↔ R${p + 1}` : '';
    if (p !== k) {
      const tmp = M[k]; M[k] = M[p]; M[p] = tmp;
    }
    // 消去主元下方各行：m = a_ik / a_kk
    const mults = [];
    for (let i = k + 1; i < n; i++) {
      const m = M[i][k] / M[k][k];
      mults.push(fmtCell(m));
      for (let j = k; j < n; j++) {
        M[i][j] -= m * M[k][j];
      }
      M[i][n] -= m * M[k][n];
    }
    const multText = mults.length ? `，乘数 m = [${mults.join(', ')}]` : '';
    this.setData({
      col: k + 1,
      history: this.data.history.concat([
        `第 ${k + 1} 列：主元 R${p + 1}（|a| = ${fmtCell(Math.abs(M[k][k]))}）${swapText}${multText}`
      ]),
      hint: manual && !isMax
        ? '⚠️ 所选主元非本列最大：数学上仍可解，但数值稳定性差（列主元策略应取 |a| 最大者）'
        : ''
    });
  },

  // 每步消元后的状态推进：全部列完成则进入回代
  afterElimStep() {
    if (this.data.col >= this.data.n - 1) {
      this.setData({ stage: 'back' });
    }
    this.refreshDisplay();
  },

  // 自动演示：剩余列一律取列主元（|a| 最大）消元，并完成全部回代
  onAutoRun() {
    if (this.data.stage === 'elim') {
      const n = this.data.n;
      while (this.data.col < n - 1) {
        const k = this.data.col;
        const M = this._M;
        let p = k;
        for (let i = k + 1; i < n; i++) {
          if (Math.abs(M[i][k]) > Math.abs(M[p][k])) p = i;
        }
        if (Math.abs(M[p][k]) < 1e-12) {
          wx.showToast({ title: '矩阵奇异，无法继续消元', icon: 'none' });
          this.refreshDisplay();
          return;
        }
        this.eliminateColumn(k, p, false);
      }
      this.setData({ stage: 'back', hint: '' });
    }
    while (this.data.stage === 'back') {
      this.backOneStep();
    }
    this.refreshDisplay();
  },

  // 回代下一步：从最后一个未知量开始逐个求出
  onBackStep() {
    if (this.data.stage !== 'back') return;
    this.backOneStep();
    this.refreshDisplay();
  },

  // 计算下一个回代未知量并生成公式行：x_i = (b_i − Σ_{j>i} a_ij·x_j) / a_ii
  backOneStep() {
    const M = this._M;
    const n = this.data.n;
    const i = n - 1 - this.data.backLines.length;
    if (i < 0) return;
    if (!this._x) this._x = new Array(n);
    let s = M[i][n];
    const parts = [];
    for (let j = i + 1; j < n; j++) {
      if (Math.abs(M[i][j]) >= 1e-12) {
        parts.push(`${fmtCell(M[i][j])}·${fmtCell(this._x[j])}`);
        s -= M[i][j] * this._x[j];
      }
    }
    const xi = s / M[i][i];
    this._x[i] = xi;
    const rhs = parts.length ? `(${fmtCell(M[i][n])} − (${parts.join(' + ')}))` : `${fmtCell(M[i][n])}`;
    const backLines = this.data.backLines.concat([
      { k: 'b' + i, text: `x${sub(i + 1)} = ${rhs} / ${fmtCell(M[i][i])} = ${fmtCell(xi)}` }
    ]);
    // 全部求出 → 进入结果阶段，给出待填写的解向量输入框
    if (backLines.length === n) {
      const inputs = [];
      for (let d = 0; d < n; d++) {
        inputs.push({ label: `x${sub(d + 1)}`, value: '' });
      }
      this.setData({
        backLines,
        stage: 'done',
        finished: true,
        solutionInputs: inputs,
        solutionText: `x = (${this._x.map(v => fmtCell(v)).join(', ')})`,
        chartValues: this.snapshotChartValues()
      });
    } else {
      this.setData({ backLines, chartValues: this.snapshotChartValues() });
    }
  },

  // 学生填写解向量
  onSolutionInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`solutionInputs[${idx}].value`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, finished, solutionInputs, config } = this.data;
    if (!finished || !solutionInputs.length) {
      wx.showToast({ title: '请先完成消元与回代', icon: 'none' });
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
          level_id: parseInt(levelId, 10) || 13,
          experiment: 'gauss_elim',
          readings: [{ solution }],
          // 携带实际出题参数：换题时后端按此方程组重算真值解（契约 §12）
          user_config: {
            system_key: config.system_key
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
      console.error('[gauss-elim] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
