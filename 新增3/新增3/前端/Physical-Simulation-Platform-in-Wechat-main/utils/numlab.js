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
    desc: '病态矩阵（教学选主元）：不选主元时精度急剧损失，解为全 1',
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
    desc: '一维泊松方程差分三对角（对称正定），SOR 主场：雅可比 114 步不收敛、ω=1 赛德尔 61 步、最优 ω≈1.4 仅 19 步',
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
