// utils/numlab.js - 数值分析实验函数库
// 与后端 internal/service/scoring.go 的 numFuncs 一一对应：
// 后端按 func_key 现算评分真值（不落库），前端仅用本库绘图、演示迭代过程。
// 新增函数须与后端同步：scoring.go 的 numFuncs + docs/api-contract.md 附录登记。

// 被测函数及其解析导数（牛顿迭代用）
export const numFuncs = {
  cubic_1: {
    label: 'f(x) = x³ − x − 1',
    f: function (x) { return x * x * x - x - 1; },
    df: function (x) { return 3 * x * x - 1; }
  },
  cos_x_x: {
    label: 'f(x) = cos x − x',
    f: function (x) { return Math.cos(x) - x; },
    df: function (x) { return -Math.sin(x) - 1; }
  },
  sqrt_2: {
    label: 'f(x) = x² − 2',
    f: function (x) { return x * x - 2; },
    df: function (x) { return 2 * x; }
  }
};

// 各函数默认出题参数（与后端 migration 006/007 种子一致）。
// 换题时随 user_config 提交（func_key + interval / x0），后端按提交参数重算真值。
export const funcPresets = {
  cubic_1: { interval: [1.0, 1.5], x0: 1.5 },
  cos_x_x: { interval: [0.0, 1.0], x0: 1.0 },
  sqrt_2: { interval: [1.0, 1.5], x0: 2.0 }
};

// ===== 自定义「最大迭代步数」的可设置范围 =====
// 下限统一为 0（0 步即不迭代，直接读初始估计值：二分取区间中点、牛顿取 x₀）。
// 上限按函数与方法各自的收敛特性给出，页面侧还要再与后端 config.max_iterations 取较小值：
// 牛顿法后端校验 iterations∈[1, max_iterations] 且该字段不可被 user_config 覆盖，
// 上限超过它会导致提交被拒。

// 二分法上限：达到容差需 ceil(log2((b−a)/tol)) 步（默认区间下 cubic_1/sqrt_2 宽 0.5 → 19 步，
// cos_x_x 宽 1.0 → 20 步）；再往后区间宽度落到双精度可分辨极限，继续二分不再改变结果。
// 故取「理论步数 + 12 步观察余量」，硬顶 52 步，并随学生自选的区间宽度变化。
export function bisectStepLimit(interval, tolerance) {
  const width = Math.abs(Number(interval[1]) - Number(interval[0]));
  const tol = Number(tolerance) > 0 ? Number(tolerance) : 1e-6;
  if (!isFinite(width) || width <= 0) return 31;
  const need = Math.ceil(Math.log(width / tol) / Math.LN2);
  return Math.max(1, Math.min(52, (need > 0 ? need : 1) + 12));
}

// 牛顿法上限：二次收敛，好初值 4~6 步即够；坏初值从 f′→0 附近出发时首步会被切线抛到极远，
// 之后大致每步减半地回到根附近，所需步数按各函数最危险的初值估：
//   sqrt_2  f′=2x，x₀≈0 时 |x₁| 可达 1e8 量级 → 约 27 步才回到根附近，给 40
//   cubic_1 f′=3x²−1 在 x≈±0.577 处为零，抛出幅度小得多 → 给 30
//   cos_x_x |f′|=|sin x+1|∈[0,2]，全局性质温和、几乎必收敛 → 给 25
export const newtonStepLimits = {
  cubic_1: 30,
  cos_x_x: 25,
  sqrt_2: 40
};

export function newtonStepLimit(funcKey) {
  return newtonStepLimits[funcKey] || 30;
}

// ===== 高斯消去法（L13）：线性方程组库 =====
// 与后端 internal/service/scoring.go 的 linSystems 一一对应，A/b 数值须逐字同步：
// 后端按 system_key 用列主元高斯现算真值解（不落库），前端仅用本库出题、渲染、演示消元。
// 换题时随 user_config 提交 {system_key}，后端按提交的方程组重算真值（契约 §12）。
// label/desc 为前端展示文案，不参与评分。
export const linSystems = {
  sys3_basic: {
    label: '经典 3 阶例题',
    desc: '良态方程组，解恰为整数，手工消去即可完成',
    A: [[2, 1, -1], [-3, -1, 2], [-2, 1, 2]],
    b: [8, -11, -3]
  },
  sys4_tridiag: {
    label: '三对角 4 阶',
    desc: '弹簧链/热传导差分模型，对角占优、良态',
    A: [[4, 1, 0, 0], [1, 4, 1, 0], [0, 1, 4, 1], [0, 0, 1, 4]],
    b: [3, -1, 7.5, 4]
  },
  sys3_diagdom: {
    label: '对角占优 3 阶',
    desc: '严格对角占优（迭代法必收敛），解恰为整数',
    A: [[10, 2, 1], [1, 10, 2], [2, 1, 10]],
    b: [17, 27, 34]
  },
  hilbert4: {
    label: 'Hilbert 病态 4 阶',
    desc: '病态矩阵（教学选主元）：条件数大、放大舍入误差，阶数越高越明显，本题解为全 1',
    A: [
      [1, 1 / 2, 1 / 3, 1 / 4],
      [1 / 2, 1 / 3, 1 / 4, 1 / 5],
      [1 / 3, 1 / 4, 1 / 5, 1 / 6],
      [1 / 4, 1 / 5, 1 / 6, 1 / 7]
    ],
    b: [25 / 12, 77 / 60, 57 / 60, 319 / 420] // 行和，故解为全 1
  },
  sys6_poisson: {
    label: '泊松方程 6 阶',
    desc: '一维泊松方程差分三对角（对称正定），SOR 主场：雅可比需 114 步（页面 100 步上限内跑不收敛）、ω=1 赛德尔 61 步、最优 ω≈1.4 仅 19 步',
    A: [
      [2, -1, 0, 0, 0, 0],
      [-1, 2, -1, 0, 0, 0],
      [0, -1, 2, -1, 0, 0],
      [0, 0, -1, 2, -1, 0],
      [0, 0, 0, -1, 2, -1],
      [0, 0, 0, 0, -1, 2]
    ],
    b: [1, 0, 0, 0, 0, 1] // 边界值驱动，故解为全 1
  }
};

// luDecompose Doolittle 三角分解 A=LU（不选主元，与后端 scoring.go 同式）：
// L 单位下三角（乘数 m_ik），U 上三角（消去结果）；返回 {L, U, lCol}，
// lCol=乘子按行序 l21,l31,l32,...（长度 n(n-1)/2）。主元≈0 抛错（LU 中断）。
// 题库 linSystems 五题均无需选主元；自定义阵可能中断——与列主元高斯的差异点。
export function luDecompose(A) {
  const n = A.length;
  if (!n) throw new Error('A 为空');
  let maxAbs = 0;
  const U = A.map(row => {
    if (row.length !== n) throw new Error('A 不是方阵');
    return row.map(Number);
  });
  for (const row of U) {
    for (const v of row) {
      if (!isFinite(v)) throw new Error('A 含 NaN/Inf');
      maxAbs = Math.max(maxAbs, Math.abs(v));
    }
  }
  const L = Array.from({ length: n }, (_, i) =>
    Array.from({ length: n }, (_, j) => (i === j ? 1 : 0)));
  const lCol = [];
  const singTol = 1e-12 * maxAbs; // 奇异阈值随矩阵元素量级缩放（与后端同款）
  for (let k = 0; k < n; k++) {
    const piv = Math.abs(U[k][k]);
    if (!isFinite(piv) || piv <= singTol) {
      throw new Error(`第 ${k + 1} 步主元≈0，不选主元的 LU 分解中断`);
    }
    for (let i = k + 1; i < n; i++) {
      const m = U[i][k] / U[k][k];
      L[i][k] = m;
      lCol.push(m);
      for (let j = k; j < n; j++) {
        U[i][j] -= m * U[k][j];
      }
    }
  }
  return { L, U, lCol };
}

// luSolve LU 分解路径求解 Ax=b：Doolittle 分解 + 前代 Ly=b + 回代 Ux=y
// （与后端 luSolve 同式；与列主元高斯在精确算术下同解）。
export function luSolve(A, b) {
  const n = A.length;
  if (b.length !== n) throw new Error('A 与 b 维度不匹配');
  const { L, U } = luDecompose(A);
  const y = new Array(n);
  for (let i = 0; i < n; i++) {
    let s = b[i];
    for (let j = 0; j < i; j++) s -= L[i][j] * y[j];
    y[i] = s;
  }
  const x = new Array(n);
  for (let i = n - 1; i >= 0; i--) {
    let s = y[i];
    for (let j = i + 1; j < n; j++) s -= U[i][j] * x[j];
    x[i] = s / U[i][i];
    if (!isFinite(x[i])) throw new Error('回代结果非有限值');
  }
  return x;
}

// ===== 追赶法（L28）：三对角方程组题库 =====
// 与后端 internal/service/scoring.go 的 triSystems 一一对应，a/b/c/d 须逐字同步：
// a 下对角（长 n-1）、b 主对角（长 n）、c 上对角（长 n-1）、d 右端（长 n）。
// 后端按 system_key 用追赶法现算真值解（不落库），前端仅用本库出题、渲染、演示追/赶。
// label/desc 为前端展示文案，不参与评分。
export const triSystems = {
  tri1_basic: {
    label: '手算例题 4 阶',
    desc: '主对角 (2,3,4,5)，解恰 (1,2,3,4)，m 与 beta 全手算',
    a: [1, 2, 1],
    b: [2, 3, 4, 5],
    c: [1, 1, 1],
    d: [4, 10, 20, 23]
  },
  tri2_poisson: {
    label: '泊松差分 5 阶',
    desc: '一维泊松差分（对称正定），beta=(2,3/2,4/3,5/4,6/5)，解全 1',
    a: [-1, -1, -1, -1],
    b: [2, 2, 2, 2, 2],
    c: [-1, -1, -1, -1],
    d: [1, 0, 0, 0, 1]
  },
  tri3_small: {
    label: '入门 3 阶',
    desc: '严格对角占优，解恰 (1,1,1)',
    a: [1, 1],
    b: [4, 4, 4],
    c: [2, 2],
    d: [6, 7, 5]
  },
  tri4_spline: {
    label: '三弯矩阵 6 阶',
    desc: 'L25 三次样条的三弯矩系数阵 [2,1; 1,4,1; ...; 1,2]，解全 1',
    a: [1, 1, 1, 1, 1],
    b: [2, 4, 4, 4, 4, 2],
    c: [1, 1, 1, 1, 1],
    d: [3, 6, 6, 6, 6, 3]
  }
};

// thomasChase 追赶法解三对角方程组（与后端 scoring.go thomasChase 同式）：
// 追（Crout 消元）beta_1=b_1；m_i=a_{i-1}/beta_{i-1}，beta_i=b_i-m_i*c_{i-1}，
// d_i<-d_i-m_i*d_{i-1}。赶（回代）x_n=d_n/beta_n；x_i=(d_i-c_i*x_{i+1})/beta_i。
// 返回 {x, m, beta, dd}（dd=更新后的右端，教学展示用）。beta≈0 抛错（追赶中断，
// 适用条件：对角占优或对称正定）。
export function thomasChase(a, b, c, d) {
  const n = b.length;
  if (!n) throw new Error('b 为空');
  if (a.length !== n - 1 || c.length !== n - 1 || d.length !== n) {
    throw new Error('三对角维度不匹配');
  }
  let maxAbs = 0;
  for (const arr of [a, b, c, d]) {
    for (const v of arr) {
      if (!isFinite(v)) throw new Error('三对角元素含 NaN/Inf');
      maxAbs = Math.max(maxAbs, Math.abs(v));
    }
  }
  const singTol = 1e-12 * maxAbs; // 奇异阈值随元素量级缩放（与后端同款）
  const m = new Array(n - 1);
  const beta = new Array(n);
  const dd = d.map(Number);
  beta[0] = b[0];
  if (Math.abs(beta[0]) <= singTol) {
    throw new Error('第 1 步 beta≈0，追赶法中断');
  }
  for (let i = 1; i < n; i++) {
    const mm = a[i - 1] / beta[i - 1];
    if (!isFinite(mm)) throw new Error('追的过程数值溢出');
    m[i - 1] = mm;
    beta[i] = b[i] - mm * c[i - 1];
    dd[i] = d[i] - mm * dd[i - 1];
    if (!isFinite(beta[i]) || !isFinite(dd[i]) || Math.abs(beta[i]) <= singTol) {
      throw new Error(`第 ${i + 1} 步 beta≈0，追赶法中断`);
    }
  }
  const x = new Array(n);
  x[n - 1] = dd[n - 1] / beta[n - 1];
  if (!isFinite(x[n - 1])) throw new Error('赶的过程数值溢出');
  for (let i = n - 2; i >= 0; i--) {
    x[i] = (dd[i] - c[i] * x[i + 1]) / beta[i];
    if (!isFinite(x[i])) throw new Error('赶的过程数值溢出');
  }
  return { x, m, beta, dd };
}

// ===== 平方根法（L29）：对称正定方程组题库 =====
// 与后端 internal/service/scoring.go 的 spdSystems 一一对应，A/b 须逐字同步：
// 后端按 system_key 用平方根法（Cholesky）现算真值解（不落库），
// 前端仅用本库出题、渲染、演示分解/前代/回代。label/desc 为前端展示文案，不参与评分。
export const spdSystems = {
  spd1_2x2: {
    label: '入门 2 阶',
    desc: 'L=[[2,0],[1,2]]，y=(3,2)，解恰 (1,1)',
    A: [[4, 2], [2, 5]],
    b: [6, 7]
  },
  spd2_min3: {
    label: 'min(i,j) 阵 3 阶',
    desc: 'L=下三角全 1，y=(6,5,3)，解恰 (1,2,3)',
    A: [[1, 1, 1], [1, 2, 2], [1, 2, 3]],
    b: [6, 11, 14]
  },
  spd3_tridiag3: {
    label: '三对角 SPD 3 阶',
    desc: 'L28 呼应：三对角对称正定，y=(3,3,3)，解恰 (1,1,1)',
    A: [[4, 2, 0], [2, 5, 2], [0, 2, 10]],
    b: [6, 9, 12]
  },
  spd4_min4: {
    label: 'min(i,j) 阵 4 阶',
    desc: 'L=下三角全 1，y=(4,3,2,1)，解全 1',
    A: [[1, 1, 1, 1], [1, 2, 2, 2], [1, 2, 3, 3], [1, 2, 3, 4]],
    b: [4, 7, 9, 10]
  }
};

// choleskySolve 平方根法解对称正定方程组（与后端 scoring.go choleskySolve 同式）：
// A = L·Lᵀ 按行分解 l_ii=sqrt(a_ii-Σ l_ik²)，l_ij=(a_ij-Σ l_ik·l_jk)/l_jj（j<i）；
// 前代 y_i=(b_i-Σ_{k<i} l_ik·y_k)/l_ii；回代 x_i=(y_i-Σ_{k>i} l_ki·x_k)/l_ii。
// 返回 {x, y, L}。非对称/根号内≤0（非正定）抛错（平方根法中断，
// 适用条件：对称正定——与列主元高斯的差异点）。
export function choleskySolve(A, b) {
  const n = A.length;
  if (!n) throw new Error('A 为空');
  let maxAbs = 0;
  for (let i = 0; i < n; i++) {
    if (A[i].length !== n) throw new Error('A 不是方阵');
    for (const v of A[i]) {
      if (!isFinite(v)) throw new Error('A 含 NaN/Inf');
      maxAbs = Math.max(maxAbs, Math.abs(v));
    }
  }
  const symTol = 1e-9 * Math.max(maxAbs, 1);
  for (let i = 0; i < n; i++) {
    for (let j = 0; j < i; j++) {
      if (Math.abs(A[i][j] - A[j][i]) > symTol) throw new Error('矩阵须对称');
    }
  }
  if (b.length !== n) throw new Error('A 与 b 维度不匹配');
  const pdTol = 1e-12 * maxAbs; // 正定阈值随元素量级缩放（与后端同款）
  const L = Array.from({ length: n }, () => new Array(n).fill(0));
  for (let i = 0; i < n; i++) {
    for (let j = 0; j <= i; j++) {
      let s = A[i][j];
      for (let k = 0; k < j; k++) s -= L[i][k] * L[j][k];
      if (i === j) {
        if (s <= pdTol) throw new Error(`第 ${i + 1} 步根号内≤0，矩阵非正定，平方根法中断`);
        L[i][i] = Math.sqrt(s);
      } else {
        L[i][j] = s / L[j][j];
      }
    }
  }
  const y = new Array(n);
  for (let i = 0; i < n; i++) {
    let s = b[i];
    for (let k = 0; k < i; k++) s -= L[i][k] * y[k];
    y[i] = s / L[i][i];
    if (!isFinite(y[i])) throw new Error('前代结果非有限值');
  }
  const x = new Array(n);
  for (let i = n - 1; i >= 0; i--) {
    let s = y[i];
    for (let k = i + 1; k < n; k++) s -= L[k][i] * x[k];
    x[i] = s / L[i][i];
    if (!isFinite(x[i])) throw new Error('回代结果非有限值');
  }
  return { x, y, L };
}

// fmtNum 数值展示：常规量 7 位有效数字，过大/过小用指数记法
export function fmtNum(v, digits) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  const d = digits || 7;
  if (v === 0) return '0';
  const abs = Math.abs(v);
  if (abs >= 0.001 && abs < 1e7) {
    return String(Number(v.toPrecision(d)));
  }
  return v.toExponential(3);
}

// 严格数字解析：输入框粘贴 "12abc"、"1.2.3" 这类垃圾时，parseFloat 会静默截断成
// 12 / 1.2 并被当作合法答案；这里整串匹配十进制格式（可选符号、整数/小数），不合法返回 null。
export function parseNum(text) {
  if (text === null || text === undefined) return null;
  const t = String(text).trim();
  if (!/^[+-]?(?:\d+\.?\d*|\.\d+)$/.test(t)) return null;
  const v = Number(t);
  return isFinite(v) ? v : null;
}

// 严格整数解析：parseInt("1e2") 会得 1、"12abc" 得 12，整串必须是整数
export function parseIntStrict(text) {
  if (text === null || text === undefined) return null;
  const t = String(text).trim();
  if (!/^[+-]?\d+$/.test(t)) return null;
  const v = parseInt(t, 10);
  return isFinite(v) ? v : null;
}

// ===== 拉格朗日插值（L16）：插值题库 =====
// 与后端 internal/service/scoring.go 的 interpProblems 一一对应，xs/ys/xEval 须逐字同步：
// 后端按 problem_key 现算插值真值 P(x_eval)（不落库），前端仅用本库出题、绘图。
// 换题时随 user_config 提交 {problem_key}，后端按提交的题重算真值（契约 §12）。
// f 为原函数（仅画图对照插值误差/龙格现象，不参与评分，不上传）；
// xMin/xMax 为画图横轴范围，不参与评分。
export const interpProblems = {
  sin_quad: {
    label: 'sin 三点二次插值',
    desc: '节点 0、π/4、π/2，求 P(π/6)；基函数值恰为 2/9、8/9、−1/9',
    fLabel: 'f(x) = sin x',
    f: function (x) { return Math.sin(x); },
    xs: [0, 0.7853981633974483, 1.5707963267948966],
    ys: [0, 0.7071067811865476, 1],
    xEval: 0.5235987755982988,
    xMin: -0.15,
    xMax: 1.75
  },
  sqrt_table: {
    label: '开方表二次插值',
    desc: '开方表节点 100、121、144，求 P(115) 近似 √115（教材经典应用）',
    fLabel: 'f(x) = √x',
    f: function (x) { return Math.sqrt(x); },
    xs: [100, 121, 144],
    ys: [10, 11, 12],
    xEval: 115,
    xMin: 90,
    xMax: 155
  },
  runge_5: {
    label: '龙格现象五点插值',
    desc: 'f=1/(1+25x²) 等距五节点，P 在端部剧烈振荡——等距高次插值的陷阱',
    fLabel: 'f(x) = 1/(1+25x²)',
    f: function (x) { return 1 / (1 + 25 * x * x); },
    xs: [-1, -0.5, 0, 0.5, 1],
    ys: [1 / 26, 4 / 29, 1, 4 / 29, 1 / 26],
    xEval: 0.8,
    xMin: -1.05,
    xMax: 1.05
  }
};

// lagrangeP 拉格朗日插值多项式求值（画 P 曲线用，与后端 lagrangeEval 同式）：
// P(x) = Σ_i y_i·Π_{j≠i}(x−x_j)/(x_i−x_j)
export function lagrangeP(xs, ys, x) {
  let v = 0;
  for (let i = 0; i < xs.length; i++) {
    let li = 1;
    for (let j = 0; j < xs.length; j++) {
      if (j !== i) li *= (x - xs[j]) / (xs[i] - xs[j]);
    }
    v += ys[i] * li;
  }
  return v;
}

// interpBasis 第 i 个基函数在 x 处的值：L_i(x) = Π_{j≠i}(x−x_j)/(x_i−x_j)
// （页面上分步演示「先算基函数再加权求和」用；xs/ys 须与后端题库逐字同步）
export function interpBasis(xs, i, x) {
  let li = 1;
  for (let j = 0; j < xs.length; j++) {
    if (j !== i) li *= (x - xs[j]) / (xs[i] - xs[j]);
  }
  return li;
}

// nevilleEval 逐次线性插值（Neville 递推，L18）求插值多项式在 xEval 处的值：
// 与后端 scoring.go 的 nevilleEval 同式（与 lagrangeP 评同一插值多项式，
// 两路结果数学上相等，后端单测交叉验证）。P_{i,0}=y_i，
// P_{i,k} = ((x−x_{i−k})·P_{i,k−1} − (x−x_i)·P_{i−1,k−1}) / (x_i−x_{i−k})
// （仅自查/画对照用，评分真值由后端现算；xs/ys 须与后端题库逐字同步）
export function nevilleEval(xs, ys, xEval) {
  const n = xs.length;
  const col = ys.slice();
  for (let k = 1; k < n; k++) {
    for (let i = n - 1; i >= k; i--) {
      col[i] = ((xEval - xs[i - k]) * col[i] - (xEval - xs[i]) * col[i - 1]) / (xs[i] - xs[i - k]);
    }
  }
  return col[n - 1];
}

// ===== 复合辛普森积分（L17）：积分题库 =====
// 与后端 internal/service/scoring.go 的 integProblems 一一对应，a/b/n 须逐字同步：
// 后端按 problem_key 现算辛普森真值 S_n（不落库），前端仅用本库出题、绘图。
// 换题/换区间/换 n 时随 user_config 提交，后端按提交参数重算真值（契约 §12）。
// 评分真值是「方法的输出」S_n 而非精确积分 I（同 L16 真值=P 的设计）：
// 抄精确积分被截断误差压住（sin_pi 默认 n=2 抄 I=2 约 42 分不过线）。
// f 为被积函数（画图/造节点表用，不上传）；label/desc/xMin/xMax 不参与评分。
export const integProblems = {
  sin_pi: {
    label: 'sin 单次辛普森',
    desc: '∫₀^π sin x dx，I=2 已知可验证；n=2 时 S₂=(π/6)(0+4·1+0)=2π/3',
    fLabel: 'f(x) = sin x',
    f: function (x) { return Math.sin(x); },
    a: 0, b: Math.PI, n: 2,
    xMin: -0.3, xMax: 3.45
  },
  erf_gauss: {
    label: '高斯误差函数',
    desc: '∫₀¹ e^(−x²) dx 无初等闭式（含 erf），只能数值积分；n=4',
    fLabel: 'f(x) = e^(−x²)',
    f: function (x) { return Math.exp(-x * x); },
    a: 0, b: 1, n: 4,
    xMin: -0.3, xMax: 1.3
  },
  sqrt_x: {
    label: '端点奇异的 √x',
    desc: 'f′(0) 发散，辛普森四阶收敛假设被破坏、收敛慢；n=8 时 S₈≈0.663079 vs I=2/3',
    fLabel: 'f(x) = √x',
    f: function (x) { return Math.sqrt(x); },
    a: 0, b: 1, n: 8,
    xMin: -0.05, xMax: 1.1
  }
};

// compositeSimpson 复合辛普森 S_n（与后端同式；仅画图对照用，评分真值由后端现算）：
// S_n = (h/3)·[f(a) + 4·Σ奇下标 f + 2·Σ偶下标 f + f(b)]，h=(b−a)/n，n 为偶数
export function compositeSimpson(f, a, b, n) {
  const h = (b - a) / n;
  let odd = 0, even = 0;
  for (let i = 1; i < n; i++) {
    const y = f(a + i * h);
    if (i % 2 === 1) odd += y; else even += y;
  }
  return h / 3 * (f(a) + 4 * odd + 2 * even + f(b));
}

// ===== 龙贝格积分（L19）：梯形逐次减半 + 理查逊外推 =====
// 与后端 scoring.go 的 rombergTColumn/rombergEval 同式（评分真值由后端现算），
// 仅自查/画对照用，不参与评分。
// rombergTColumn: T_0=(b−a)/2·[f(a)+f(b)]，T_m=T_{m−1}/2+h_m·Σf(新节点)，h_m=(b−a)/2^m
// （每层只算上一层分点中间的奇数位节点，f 值不重复计算）
export function rombergTColumn(f, a, b, k) {
  const t = [(b - a) * (f(a) + f(b)) / 2];
  for (let m = 1; m <= k; m++) {
    const steps = Math.pow(2, m);
    const h = (b - a) / steps;
    let s = 0;
    for (let i = 1; i < steps; i += 2) s += f(a + i * h);
    t[m] = t[m - 1] / 2 + h * s;
  }
  return t;
}

// rombergEval 龙贝格终值 T_k^(k)：T 列做 k 轮理查逊外推
// T_i^(j) = (4^j·T_i^(j−1) − T_{i−1}^(j−1)) / (4^j − 1)，原地自下而上收缩。
// 外推第 1 列恰为复合辛普森 S_{2^m}（与 compositeSimpson 同式，后端单测交叉验证）
export function rombergEval(f, a, b, k) {
  const t = rombergTColumn(f, a, b, k);
  for (let j = 1; j <= k; j++) {
    const factor = Math.pow(4, j);
    for (let i = k; i >= j; i--) {
      t[i] = (factor * t[i] - t[i - 1]) / (factor - 1);
    }
  }
  return t[k];
}

// ===== 欧拉法（L20/L21）：常微分方程初值问题 y′ = f(x,y)，y(a)=y0 =====

// odeProblems 常微分方程题库（与后端 scoring.go 的 odeProblems 逐字同步）：
// f 为右端函数（画斜率场/自查用），exact 仅前端画图对照参考线用、
// 评分真值由后端现算（真值=方法输出 y_N，精确解记审计 ref_Y 不打分）。
export const odeProblems = {
  exp_growth: {
    label: '指数增长 y′=y',
    desc: 'y′=y, y(0)=1，精确解 e^x；显式欧拉 n=4 恰 (1.25)⁴=2.44140625 全列手算闭式',
    fLabel: 'f(x, y) = y',
    f: function (x, y) { return y; },
    exact: function (x) { return Math.exp(x); },
    a: 0, b: 1, y0: 1, n: 4,
    xMin: -0.15, xMax: 1.15
  },
  decay_sq: {
    label: '高斯衰减 y′=−2xy',
    desc: 'y′=−2xy, y(0)=1，精确解 e^(−x²)（与 L17 erf_gauss 同源）；f 显含 x 与 y',
    fLabel: 'f(x, y) = −2xy',
    f: function (x, y) { return -2 * x * y; },
    exact: function (x) { return Math.exp(-x * x); },
    a: 0, b: 1, y0: 1, n: 4,
    xMin: -0.15, xMax: 1.15
  },
  stiff_decay: {
    label: '刚性方程 y′=−10y',
    desc: 'y′=−10y, y(0)=1，精确解 e^(−10x)；h=0.25 时 hλ=−2.5 超出两法稳定区间：显式欧拉变号振荡发散，改进欧拉同号单调发散（观察题）',
    fLabel: 'f(x, y) = −10y',
    f: function (x, y) { return -10 * y; },
    exact: function (x) { return Math.exp(-10 * x); },
    a: 0, b: 1, y0: 1, n: 4,
    xMin: -0.05, xMax: 1.05
  }
};

// eulerExplicit 显式（前进）欧拉列 y_0..y_N（与后端同式，仅画折线/自查用，
// 评分真值由后端现算）：y_{n+1} = y_n + h·f(x_n, y_n)
export function eulerExplicit(f, a, b, y0, n) {
  const ys = [y0];
  const h = (b - a) / n;
  for (let i = 0; i < n; i++) {
    const x = a + i * h;
    ys.push(ys[i] + h * f(x, ys[i]));
  }
  return ys;
}

// eulerImproved 改进欧拉（预测-校正/Heun）列 y_0..y_N（与后端同式）：
// 预测 y_p = y_n + h·f(x_n, y_n)；校正 y_{n+1} = y_n + h/2·[f(x_n,y_n) + f(x_{n+1},y_p)]
export function eulerImproved(f, a, b, y0, n) {
  const ys = [y0];
  const h = (b - a) / n;
  for (let i = 0; i < n; i++) {
    const x = a + i * h;
    const yp = ys[i] + h * f(x, ys[i]);
    ys.push(ys[i] + h / 2 * (f(x, ys[i]) + f(x + h, yp)));
  }
  return ys;
}

// implicitEulerColumn 隐式（后退）欧拉列 y_0..y_N（与后端同式）：
// y_{n+1} = y_n + h·f(x_{n+1}, y_{n+1})，未知量在等式两侧；题库 odeProblems
// 均为线性齐次 y'=a(x)·y（f(x,y)=y·f(x,1)），闭式解 y_{n+1} = y_n/(1−h·f(x_{n+1},1))。
// 稳定性函数 R(hλ)=1/(1−hλ)：绝对稳定域含整个左半平面（A 稳定），
// 与显式欧拉圆盘 |1+hλ|≤1 互补——stiff_decay 上显式发散处隐式单调收敛。
// 前端仅用于学生自查/画精确对照（L26 euler_implicit 页），真值以后端重算为准。
export function implicitEulerColumn(f, a, b, y0, n) {
  const ys = [y0];
  const h = (b - a) / n;
  for (let i = 0; i < n; i++) {
    const x1 = a + (i + 1) * h;
    ys.push(ys[i] / (1 - h * f(x1, 1)));
  }
  return ys;
}

// rk4Column 四阶龙格-库塔列 y_0..y_N（与后端同式）：
// K1=f(x_n,y_n)；K2=f(x_n+h/2, y_n+h/2·K1)；K3=f(x_n+h/2, y_n+h/2·K2)；K4=f(x_n+h, y_n+h·K3)；
// y_{n+1} = y_n + h/6·(K1+2K2+2K3+K4)，全局截断误差 O(h⁴)。
// 前端仅用于学生自查/画精确对照（L22 runge_kutta_ode 页），真值以后端重算为准。
export function rk4Column(f, a, b, y0, n) {
  const ys = [y0];
  const h = (b - a) / n;
  for (let i = 0; i < n; i++) {
    const x = a + i * h;
    const k1 = f(x, ys[i]);
    const k2 = f(x + h / 2, ys[i] + h / 2 * k1);
    const k3 = f(x + h / 2, ys[i] + h / 2 * k2);
    const k4 = f(x + h, ys[i] + h * k3);
    ys.push(ys[i] + h / 6 * (k1 + 2 * k2 + 2 * k3 + k4));
  }
  return ys;
}

// adamsPCColumn 阿当姆斯预报-校正列 y_0..y_N（与后端同式）：
// 线性多步法 AB4 外推预报 + AM4 内插校正一次（PECE），
// 表头 y_1..y_3 用 RK4 启动（与 rk4Column 同式），全局截断误差 O(h⁴)，n 须 ≥ 4。
// 前端仅用于学生自查/画精确对照（L23 adams_pc_ode 页），真值以后端重算为准。
export function adamsPCColumn(f, a, b, y0, n) {
  const ys = [y0];
  const h = (b - a) / n;
  for (let i = 0; i < n; i++) {
    const x = a + i * h;
    if (i < 3) {
      const k1 = f(x, ys[i]);
      const k2 = f(x + h / 2, ys[i] + h / 2 * k1);
      const k3 = f(x + h / 2, ys[i] + h / 2 * k2);
      const k4 = f(x + h, ys[i] + h * k3);
      ys.push(ys[i] + h / 6 * (k1 + 2 * k2 + 2 * k3 + k4));
      continue;
    }
    const yp = ys[i] + h / 24 * (55 * f(x, ys[i]) -
      59 * f(x - h, ys[i - 1]) +
      37 * f(x - 2 * h, ys[i - 2]) -
      9 * f(x - 3 * h, ys[i - 3]));
    ys.push(ys[i] + h / 24 * (9 * f(x + h, yp) +
      19 * f(x, ys[i]) -
      5 * f(x - h, ys[i - 1]) +
      f(x - 2 * h, ys[i - 2])));
  }
  return ys;
}

// fitProblems 最小二乘拟合题库（与后端 scoring.go fitProblems 逐字同步）：
// lin_line 直线拟合（法方程手算 a=1.1、b=0.96，P(3.5)=4.46）/
// quad_parabola 二次拟合 5 点（系数恰 27/350、1/28、137/140，P(2)=711/175）/
// exp_lnw 指数模型 y=a·e^(bx) ln 线性化拟合（真值=e^(A+2.5B)=6.9103931）。
// m 拟合次数（须 0~min(5, 点数-2)，上界防拟合退化成插值）；ln=true 先取 ln y 拟合。
export const fitProblems = {
  lin_line: {
    xs: [1, 2, 3, 4],
    ys: [2, 3.1, 4, 4.9],
    m: 1,
    xEval: 3.5,
    ln: false,
    label: '直线拟合 y=a+bx',
    desc: '4 组 (x,y) 观测数据做一次多项式拟合，法方程 4a+10b=14、10a+30b=39.8 手算 a=1.1、b=0.96'
  },
  quad_parabola: {
    xs: [0, 1, 2, 3, 4],
    ys: [0.1, 1.1, 3.9, 9.2, 15.8],
    m: 2,
    xEval: 2,
    ln: false,
    label: '二次多项式拟合',
    desc: '数据≈x² 的 5 组观测做二次拟合，x_eval=2 恰落在数据点上（LSQ 靠近但不过点）'
  },
  exp_lnw: {
    xs: [0, 1, 2, 3, 4],
    ys: [1.8, 2.9, 5.2, 9.0, 16.2],
    m: 1,
    xEval: 2.5,
    ln: true,
    label: '指数模型 ln 线性化',
    desc: '细菌增长型数据拟合 y=a·e^(bx)：先取 Y=ln y 线性化，再对 (x, Y) 直线拟合 A+Bx'
  }
};

// lsqGaussSolve 列主元高斯消去解 Ax=b（与后端 gaussSolve 同式，法方程求解用）。
function lsqGaussSolve(A, b) {
  const n = A.length;
  const M = A.map((row, i) => [...row, b[i]]);
  for (let k = 0; k < n; k++) {
    let p = k;
    for (let i = k + 1; i < n; i++) {
      if (Math.abs(M[i][k]) > Math.abs(M[p][k])) p = i;
    }
    [M[k], M[p]] = [M[p], M[k]];
    for (let i = k + 1; i < n; i++) {
      const f = M[i][k] / M[k][k];
      for (let j = k; j <= n; j++) M[i][j] -= f * M[k][j];
    }
  }
  const x = new Array(n);
  for (let i = n - 1; i >= 0; i--) {
    let s = M[i][n];
    for (let j = i + 1; j < n; j++) s -= M[i][j] * x[j];
    x[i] = s / M[i][i];
  }
  return x;
}

// lsqFit 幂基最小二乘拟合（与后端同式）：法方程 [Σx^(i+j)]c=[Σx^j·y]（j=0..m），
// 列主元高斯求解，返回系数 [c_0..c_m]；ln=true 先对 y 取 ln，返回 ln 域系数。
// 前端仅用于学生自查/画拟合曲线（L24 least_squares_fit 页），真值以后端重算为准。
export function lsqFit(xs, ys, m, ln) {
  const n = xs.length;
  const powSum = new Array(2 * m + 1).fill(0);
  const xySum = new Array(m + 1).fill(0);
  for (let k = 0; k < n; k++) {
    const y = ln ? Math.log(ys[k]) : ys[k];
    let xp = 1;
    for (let p = 0; p <= 2 * m; p++) {
      powSum[p] += xp;
      if (p <= m) xySum[p] += xp * y;
      xp *= xs[k];
    }
  }
  const A = [], b = [];
  for (let j = 0; j <= m; j++) {
    A.push(Array.from({ length: m + 1 }, (_, i) => powSum[i + j]));
    b.push(xySum[j]);
  }
  return lsqGaussSolve(A, b);
}

// lsqPolyEval 幂基多项式求值 P(x)=Σ c_i x^i（霍纳嵌套）。
export function lsqPolyEval(cs, x) {
  let v = 0;
  for (let i = cs.length - 1; i >= 0; i--) v = v * x + cs[i];
  return v;
}

// ========== L15 三次样条插值（cubic_spline，v0.20）==========

// splineProblems 样条题库（与后端 scoring.go splineProblems 逐字同步）。
export const splineProblems = {
  dist_4: {
    xs: [0, 1, 2, 4], ys: [0, 2.5, 6, 20], xEval: 3,
    label: '位移观测数据（不等距 h=1,1,2）',
    desc: '位移观测数据 (0,0),(1,2.5),(2,6),(4,20)：不等距区间，三弯矩手算 M1=15/23、M2=78/23，S(3)=559/46'
  },
  sin_5: {
    xs: [0, Math.PI / 4, Math.PI / 2, 3 * Math.PI / 4, Math.PI],
    ys: [0, Math.SQRT2 / 2, 1, Math.SQRT2 / 2, 0], xEval: Math.PI / 6,
    f: Math.sin, fLabel: 'y = sin x',
    label: 'sin 五等距节点',
    desc: 'sin 五等距节点（弯矩对称 M1=M3），样条逼近光滑函数极好；抄 sin(π/6)=0.5 约 83 分过线不满分'
  },
  runge_5: {
    xs: [-1, -0.5, 0, 0.5, 1],
    ys: [1 / 26, 4 / 29, 1, 4 / 29, 1 / 26], xEval: 0.8,
    f: x => 1 / (1 + 25 * x * x), fLabel: 'y = 1/(1+25x²)',
    label: '龙格函数（与 L16 同题）',
    desc: '龙格函数五等距节点，与 L16 拉格朗日同节点同求值点三法对照：P(0.8)≈−0.379 剧烈振荡 vs 样条 S(0.8)≈−0.036；自然边界端段仍下弯'
  }
};

// splineThomas 三对角追赶法（与后端 thomasSolve 同式）：
// a 连左次对角、b 主对角、c 连右超对角、d 右端。
function splineThomas(a, b, c, d) {
  const n = b.length;
  const cp = new Array(n), dp = new Array(n);
  cp[0] = c[0] / b[0];
  dp[0] = d[0] / b[0];
  for (let i = 1; i < n; i++) {
    const m = b[i] - a[i] * cp[i - 1];
    cp[i] = c[i] / m;
    dp[i] = (d[i] - a[i] * dp[i - 1]) / m;
  }
  const x = new Array(n);
  x[n - 1] = dp[n - 1];
  for (let i = n - 2; i >= 0; i--) x[i] = dp[i] - cp[i] * x[i + 1];
  return x;
}

// splineMoments 自然边界三弯矩（与后端同式）：返回弯矩列 M_0..M_{n-1}。
// 方程 mu_i·M_{i-1} + 2M_i + lam_i·M_{i+1} = d_i（i=1..n-2，M_0=M_{n-1}=0），
// mu_i=h_i/(h_i+h_{i+1})，lam_i=1-mu_i，d_i=6·[(y_{i+1}−y_i)/h_{i+1}−(y_i−y_{i−1})/h_i]/(h_i+h_{i+1})。
export function splineMoments(xs, ys) {
  const n = xs.length;
  const M = new Array(n).fill(0);
  const k = n - 2;
  const a = new Array(k).fill(0), b = new Array(k).fill(2),
        c = new Array(k).fill(0), d = new Array(k);
  for (let i = 1; i <= n - 2; i++) {
    const h0 = xs[i] - xs[i - 1], h1 = xs[i + 1] - xs[i];
    d[i - 1] = 6 * ((ys[i + 1] - ys[i]) / h1 - (ys[i] - ys[i - 1]) / h0) / (h0 + h1);
    if (i > 1) a[i - 1] = h0 / (h0 + h1);
    if (i < n - 2) c[i - 1] = h1 / (h0 + h1);
  }
  const inner = splineThomas(a, b, c, d);
  for (let i = 1; i <= n - 2; i++) M[i] = inner[i - 1];
  return M;
}

// splineEvalAt 样条段上求值（与后端同式）：先定位段再按系数式
// S(t)=y_k+B·w+C·w²+D·w³（w=t−x_k）。前端仅用于学生自查/画样条曲线，
// 真值以后端重算为准。
export function splineEvalAt(xs, ys, M, t) {
  const n = xs.length;
  let k = 0;
  for (let i = 0; i < n - 1; i++) {
    if (t >= xs[i]) k = i;
  }
  if (k > n - 2) k = n - 2;
  const h = xs[k + 1] - xs[k], w = t - xs[k];
  const B = (ys[k + 1] - ys[k]) / h - h * (2 * M[k] + M[k + 1]) / 6;
  const C = M[k] / 2, D = (M[k + 1] - M[k]) / (6 * h);
  return ys[k] + w * (B + w * (C + w * D));
}
