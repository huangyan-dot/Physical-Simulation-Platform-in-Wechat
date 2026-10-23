// utils/fieldchart.js - ODE 斜率场公共绘制（5 个微分方程实验页共用）
// 每个网格点 (x,y) 画一段方向沿 y′=f(x,y) 的小箭头；颜色与不透明度编码斜率大小：
// 平缓 → 主蓝，中等 → 琥珀，陡峭 → 红，s=0 画小空心圆（方向水平但零斜率信息保留）。
// 配色全部取自项目既有色板，不引入新色相。

// 蓝色 #3498db → 琥珀 #f39c12 → 红 #e74c3c
function slopeColor(t) {
  const stops = [
    [52, 152, 219],
    [243, 156, 18],
    [231, 76, 60]
  ];
  const u = Math.max(0, Math.min(1, t));
  const [a, b] = u < 0.55
    ? [stops[0], stops[1]]
    : [stops[1], stops[2]];
  const q = u < 0.55 ? u / 0.55 : (u - 0.55) / 0.45;
  const r = Math.round(a[0] + (b[0] - a[0]) * q);
  const g = Math.round(a[1] + (b[1] - a[1]) * q);
  const bl = Math.round(a[2] + (b[2] - a[2]) * q);
  return { rgb: `rgb(${r},${g},${bl})`, alpha: 0.28 + 0.5 * u };
}

// 画一段带小翼的箭头（方向向量 dx,dy 已归一化，cx/cy 为像素中心）
function arrowSegment(ctx, cx, cy, dx, dy, half, color, alpha) {
  ctx.strokeStyle = color;
  ctx.globalAlpha = alpha;
  ctx.beginPath();
  ctx.moveTo(cx - dx * half, cy - dy * half);
  ctx.lineTo(cx + dx * half, cy + dy * half);
  ctx.stroke();
  // 箭头两翼（翼长 3.5，与 plane3d.js 坐标轴箭头同款画法）
  const ux = dx, uy = dy;
  ctx.beginPath();
  ctx.moveTo(cx + dx * half, cy + dy * half);
  ctx.lineTo(cx + dx * half - ux * 5 + uy * 3.2, cy + dy * half - uy * 5 - ux * 3.2);
  ctx.moveTo(cx + dx * half, cy + dy * half);
  ctx.lineTo(cx + dx * half - ux * 5 - uy * 3.2, cy + dy * half - uy * 5 + ux * 3.2);
  ctx.stroke();
  ctx.globalAlpha = 1;
}

// 在当前笛卡尔坐标系内画斜率场。
// o: { ctx, X, Y, view:{xMin,xMax,yMin,yMax}, mL, mT, pw, ph, f(x,y), nx, ny, half }
export function drawSlopeField(o) {
  const { ctx, X, Y, view, f } = o;
  const nx = o.nx || 13, ny = o.ny || 9, half = o.half || 7;
  const kx = o.pw / (view.xMax - view.xMin);
  const ky = o.ph / (view.yMax - view.yMin);

  // 第一遍采样 |slope|，用 95 分位做归一化上限，避免单个极端值压扁全场色差
  const mags = [];
  const grid = [];
  for (let gi = 0; gi <= nx; gi++) {
    const x = view.xMin + (view.xMax - view.xMin) * gi / nx;
    for (let gj = 0; gj <= ny; gj++) {
      const y = view.yMin + (view.yMax - view.yMin) * gj / ny;
      const s = f(x, y);
      if (isFinite(s)) { mags.push(Math.abs(s)); grid.push([x, y, s]); }
    }
  }
  if (!mags.length) return;
  const sorted = mags.slice().sort((p, q) => p - q);
  const smax = sorted[Math.floor(sorted.length * 0.95)];
  if (!(smax > 0)) {
    // 全场斜率为 0：统一画水平蓝色小箭头
    grid.forEach(([x, y]) => arrowSegment(ctx, X(x), Y(y), 1, 0, half, 'rgb(52,152,219)', 0.28));
    return;
  }

  ctx.lineWidth = 1.2;
  grid.forEach(([x, y, s]) => {
    const cx = X(x), cy = Y(y);
    if (s === 0) {
      // 零斜率：小空心圆（保留网格点信息，不与水平箭头混淆）
      ctx.strokeStyle = 'rgb(52,152,219)';
      ctx.globalAlpha = 0.3;
      ctx.beginPath();
      ctx.arc(cx, cy, 1.8, 0, Math.PI * 2);
      ctx.stroke();
      ctx.globalAlpha = 1;
      return;
    }
    let dx = kx, dy = s * ky;
    const norm = Math.sqrt(dx * dx + dy * dy);
    dx /= norm; dy /= norm;
    const col = slopeColor(Math.abs(s) / smax);
    arrowSegment(ctx, cx, cy, dx, dy, half, col.rgb, col.alpha);
  });
}
