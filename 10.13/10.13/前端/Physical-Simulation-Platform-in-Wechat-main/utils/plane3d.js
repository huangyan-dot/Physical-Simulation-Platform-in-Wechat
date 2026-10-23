// utils/plane3d.js - 三元线性方程组的三维平面可视化（纯 canvas 2D 软件投影，无 webgl 依赖）
// 教学含义：n=3 时每个方程 a₁x₁+a₂x₂+a₃x₃=b 是空间中的一张平面，
//   方程组的解 = 三张平面的交点；迭代序列 x⁽ᵏ⁾ 是空间中飞向该交点的轨迹。
// 仅提供纯函数/纯绘制，交互状态（yaw/pitch/zoom）由调用组件持有。

// 平面/坐标轴配色（平面 i 与坐标轴 xᵢ₊₁ 同色）
export const PLANE_STROKES = ['#e74c3c', '#2ecc71', '#3498db'];
export const PLANE_FILLS = ['rgba(231,76,60,0.13)', 'rgba(46,204,113,0.13)', 'rgba(52,152,219,0.13)'];

function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

// 世界坐标 -> 相机坐标：先绕 y 轴 yaw，再绕 x 轴 pitch（正交投影，数学图无近大远小）
export function transform3d(p, yaw, pitch) {
  const x = p[0], y = p[1], z = p[2];
  const cy = Math.cos(yaw), sy = Math.sin(yaw);
  const x1 = x * cy + z * sy;
  const z1 = -x * sy + z * cy;
  const cp = Math.cos(pitch), sp = Math.sin(pitch);
  return {
    x: x1,
    y: y * cp - z1 * sp,
    d: y * sp + z1 * cp
  };
}

// 立方体 [-L,L]^3 的 8 个顶点
function cubeCorners(L) {
  const out = [];
  for (let i = 0; i < 8; i++) {
    out.push([(i & 1 ? L : -L), (i & 2 ? L : -L), (i & 4 ? L : -L)]);
  }
  return out;
}

// 立方体的 12 条边（顶点位掩码：x→1, y→2, z→4；沿每条轴各 4 条）
const CUBE_EDGES = (() => {
  const edges = [];
  for (let dim = 0; dim < 3; dim++) {
    const others = [0, 1, 2].filter(d => d !== dim);
    [0, 1].forEach((s1) => {
      [0, 1].forEach((s2) => {
        const base = (s1 ? 1 << others[0] : 0) + (s2 ? 1 << others[1] : 0);
        edges.push([base, base | (1 << dim)]);
      });
    });
  }
  return edges;
})();

// 平面 a·x=b 与立方体 [-L,L]^3 的截面凸多边形（3~6 个顶点，按法向排序）
export function planePolygon(a, b, L) {
  const corners = cubeCorners(L);
  const f = (p) => a[0] * p[0] + a[1] * p[1] + a[2] * p[2] - b;
  const pts = [];
  CUBE_EDGES.forEach(([i, j]) => {
    const p0 = corners[i], p1 = corners[j];
    const f0 = f(p0), f1 = f(p1);
    const denom = f1 - f0;
    if (Math.abs(denom) < 1e-12) return;
    const t = -f0 / denom;
    if (t < -1e-7 || t > 1 + 1e-7) return;
    const p = [
      p0[0] + t * (p1[0] - p0[0]),
      p0[1] + t * (p1[1] - p0[1]),
      p0[2] + t * (p1[2] - p0[2])
    ];
    // 去重（平面过棱/顶点时会重复命中）
    if (!pts.some(q => Math.hypot(q[0] - p[0], q[1] - p[1], q[2] - p[2]) < L * 1e-5)) pts.push(p);
  });
  if (pts.length < 3) return null;

  // 在平面内构造正交基 (u,v)，按相对质心的极角排序，保证多边形顺序正确
  const nlen = Math.hypot(a[0], a[1], a[2]) || 1;
  const n = [a[0] / nlen, a[1] / nlen, a[2] / nlen];
  const axis = Math.abs(n[1]) < 0.9 ? [0, 1, 0] : [1, 0, 0];
  const cross = (p, q) => [
    p[1] * q[2] - p[2] * q[1],
    p[2] * q[0] - p[0] * q[2],
    p[0] * q[1] - p[1] * q[0]
  ];
  const norm = (p) => {
    const l = Math.hypot(p[0], p[1], p[2]) || 1;
    return [p[0] / l, p[1] / l, p[2] / l];
  };
  const dot = (p, q) => p[0] * q[0] + p[1] * q[1] + p[2] * q[2];
  const u = norm(cross(n, axis));
  const v = cross(n, u);
  const c = [0, 1, 2].map(d => pts.reduce((s, q) => s + q[d], 0) / pts.length);
  pts.sort((p, q) => {
    const ap = Math.atan2(dot(p, v) - dot(c, v), dot(p, u) - dot(c, u));
    const aq = Math.atan2(dot(q, v) - dot(c, v), dot(q, u) - dot(c, u));
    return ap - aq;
  });
  return pts;
}

// 完整场景绘制（深色背景由调用方先铺）
// opts: { ctx, w,h,mL,mT,pw,ph, A,b, yaw,pitch,zoom, path, point, showPlanes,
//         time(ms，脉冲相位), planeAlpha(0..1，平面淡入), pathReveal(0..1，轨迹生长) }
export function drawScene3d(o) {
  const { ctx, w, h, mL, mT, pw, ph, A, b, yaw, pitch } = o;
  const zoom = o.zoom || 1;
  const time = o.time || 0;
  const planeAlpha = o.planeAlpha == null ? 1 : Math.max(0, Math.min(1, o.planeAlpha));
  const pathReveal = o.pathReveal == null ? 1 : Math.max(0, Math.min(1, o.pathReveal));
  const cx = mL + pw / 2;
  const cy = mT + ph / 2;

  // 视野范围：自适应轨迹/解点量级，发散时封顶以免平面缩没
  let maxV = 0;
  (o.path || []).forEach((p) => {
    if (Array.isArray(p)) p.forEach(v => { if (isFinite(v) && Math.abs(v) > maxV) maxV = Math.abs(v); });
  });
  if (o.point) o.point.forEach(v => { if (isFinite(v) && Math.abs(v) > maxV) maxV = Math.abs(v); });
  const L = Math.min(60, Math.max(3.5, maxV * 1.5));

  const scale = Math.min(pw, ph) / (2.75 * L) * zoom;
  const P = (p) => {
    const t = transform3d(p, yaw, pitch);
    return { sx: cx + t.x * scale, sy: cy - t.y * scale, d: t.d };
  };

  // ---- 立方体边框 ----
  const corners = cubeCorners(L);
  const pc = corners.map(P);
  ctx.lineWidth = 1;
  ctx.strokeStyle = 'rgba(255,255,255,0.10)';
  CUBE_EDGES.forEach(([i, j]) => {
    ctx.beginPath();
    ctx.moveTo(pc[i].sx, pc[i].sy);
    ctx.lineTo(pc[j].sx, pc[j].sy);
    ctx.stroke();
  });

  // ---- 坐标轴（x₁ 红 / x₂ 绿 / x₃ 蓝）----
  ctx.font = '12px sans-serif';
  for (let ax = 0; ax < 3; ax++) {
    const e0 = [0, 0, 0], e1 = [0, 0, 0];
    e1[ax] = L;
    const q0 = P(e0), q1 = P(e1);
    ctx.strokeStyle = PLANE_STROKES[ax];
    ctx.fillStyle = PLANE_STROKES[ax];
    ctx.globalAlpha = 0.9;
    ctx.lineWidth = 1.6;
    ctx.beginPath();
    ctx.moveTo(q0.sx, q0.sy);
    ctx.lineTo(q1.sx, q1.sy);
    ctx.stroke();
    // 箭头
    const back = P((() => { const p = [0, 0, 0]; p[ax] = L * 0.88; return p; })());
    const dx = q1.sx - back.sx, dy = q1.sy - back.sy;
    const dl = Math.hypot(dx, dy) || 1;
    const ux = dx / dl, uy = dy / dl;
    ctx.beginPath();
    ctx.moveTo(q1.sx, q1.sy);
    ctx.lineTo(q1.sx - ux * 8 + uy * 3.5, q1.sy - uy * 8 - ux * 3.5);
    ctx.moveTo(q1.sx, q1.sy);
    ctx.lineTo(q1.sx - ux * 8 - uy * 3.5, q1.sy - uy * 8 + ux * 3.5);
    ctx.stroke();
    ctx.textAlign = 'center';
    ctx.fillText('x' + sub(ax + 1), q1.sx + ux * 12, q1.sy + uy * 12 + 4);
    ctx.globalAlpha = 1;
  }
  // 原点
  const o0 = P([0, 0, 0]);
  ctx.fillStyle = 'rgba(255,255,255,0.6)';
  ctx.beginPath();
  ctx.arc(o0.sx, o0.sy, 2, 0, Math.PI * 2);
  ctx.fill();

  // ---- 平面（按相机深度从远到近绘制）----
  const planes = [];
  if (o.showPlanes !== false && Array.isArray(A)) {
    A.forEach((row, i) => {
      const poly = planePolygon(row, b[i], L);
      if (!poly) return;
      const pp = poly.map(P);
      const depth = pp.reduce((s, q) => s + q.d, 0) / pp.length;
      planes.push({ pp, depth, i });
    });
    planes.sort((p, q) => p.depth - q.depth);
    ctx.save();
    ctx.globalAlpha = planeAlpha; // 换题/进入三维时平面整体淡入
    planes.forEach(({ pp, i }) => {
      ctx.beginPath();
      pp.forEach((q, k) => { if (k === 0) ctx.moveTo(q.sx, q.sy); else ctx.lineTo(q.sx, q.sy); });
      ctx.closePath();
      ctx.fillStyle = PLANE_FILLS[i % 3];
      ctx.fill();
      ctx.strokeStyle = PLANE_STROKES[i % 3];
      ctx.lineWidth = 1.6;
      ctx.stroke();
      // 平面编号标在质心
      const cc = pp.reduce((s, q) => ({ sx: s.sx + q.sx / pp.length, sy: s.sy + q.sy / pp.length }), { sx: 0, sy: 0 });
      ctx.fillStyle = PLANE_STROKES[i % 3];
      ctx.textAlign = 'center';
      ctx.font = 'bold 14px sans-serif';
      ctx.fillText('①②③④⑤⑥'.charAt(i), cc.sx, cc.sy + 5);
      ctx.font = '12px sans-serif';
    });
    ctx.restore();
  }

  // ---- 迭代轨迹 x⁽ᵏ⁾（飞出视野的点断线处理；pathReveal 让最新一段从上个点长出）----
  const path = o.path || [];
  if (path.length > 1) {
    const inRange = (p) => Array.isArray(p) && p.every(v => isFinite(v)) &&
      Math.max(Math.abs(p[0]), Math.abs(p[1]), Math.abs(p[2])) <= L * 1.6;
    const qs = path.map(p => (inRange(p) ? P(p) : null));
    // 有效段（端点都在视野内）编号 0..m-1；reveal 截断在最后一段内部
    const segs = [];
    for (let i = 1; i < qs.length; i++) {
      if (qs[i - 1] && qs[i]) segs.push([qs[i - 1], qs[i]]);
    }
    const fullSegs = pathReveal >= 1 ? segs.length : Math.floor(pathReveal * segs.length);
    const frac = pathReveal >= 1 ? 0 : pathReveal * segs.length - fullSegs;
    let head = null;
    ctx.strokeStyle = '#f5c542';
    ctx.lineWidth = 2;
    ctx.beginPath();
    let pen = false;
    let segIdx = -1;
    for (let i = 1; i < qs.length; i++) {
      if (!qs[i - 1] || !qs[i]) { pen = false; continue; }
      segIdx++;
      if (segIdx < fullSegs) {
        if (!pen) { ctx.moveTo(qs[i - 1].sx, qs[i - 1].sy); pen = true; }
        ctx.lineTo(qs[i].sx, qs[i].sy);
        if (segIdx === fullSegs - 1) head = qs[i];
      } else if (segIdx === fullSegs && frac > 0) {
        if (!pen) ctx.moveTo(qs[i - 1].sx, qs[i - 1].sy);
        const hx = qs[i - 1].sx + (qs[i].sx - qs[i - 1].sx) * frac;
        const hy = qs[i - 1].sy + (qs[i].sy - qs[i - 1].sy) * frac;
        ctx.lineTo(hx, hy);
        head = { sx: hx, sy: hy };
      }
    }
    ctx.stroke();
    // 历点（已长成的段），最新点由脉冲头标出
    qs.forEach((q) => {
      if (!q || (head && Math.abs(q.sx - head.sx) < 0.5 && Math.abs(q.sy - head.sy) < 0.5)) return;
      ctx.fillStyle = 'rgba(245,197,66,0.7)';
      ctx.beginPath();
      ctx.arc(q.sx, q.sy, 2.2, 0, Math.PI * 2);
      ctx.fill();
    });
    if (head) {
      const pulse = 3.2 + 1.1 * Math.sin(time / 180);
      ctx.fillStyle = '#f5c542';
      ctx.beginPath();
      ctx.arc(head.sx, head.sy, pulse, 0, Math.PI * 2);
      ctx.fill();
      ctx.strokeStyle = 'rgba(245,197,66,0.5)';
      ctx.lineWidth = 1.2;
      ctx.beginPath();
      ctx.arc(head.sx, head.sy, pulse + 3 + 2 * (0.5 + 0.5 * Math.sin(time / 220)), 0, Math.PI * 2);
      ctx.stroke();
    }
  }

  // ---- 解点（三平面交点）：核心 + 两道错相扩张光环 ----
  if (o.point && o.point.every(v => isFinite(v))) {
    const q = P(o.point);
    const sec = time / 1000;
    for (let k = 0; k < 2; k++) {
      const phase = ((sec + k * 0.8) % 1.6) / 1.6;
      ctx.strokeStyle = `rgba(255, 224, 138, ${(0.7 * (1 - phase)).toFixed(3)})`;
      ctx.lineWidth = 1.6;
      ctx.beginPath();
      ctx.arc(q.sx, q.sy, 7 + phase * 22, 0, Math.PI * 2);
      ctx.stroke();
    }
    ctx.fillStyle = '#ffe08a';
    ctx.beginPath();
    ctx.arc(q.sx, q.sy, 6, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.arc(q.sx, q.sy, 8.5, 0, Math.PI * 2);
    ctx.stroke();
    ctx.textAlign = 'left';
    ctx.font = 'bold 12px sans-serif';
    ctx.fillStyle = '#ffe08a';
    ctx.fillText('解 x*', q.sx + 10, q.sy - 8);
  }

  // ---- 图例 ----
  const legend = [];
  if (o.showPlanes !== false && Array.isArray(A)) {
    A.forEach((row, i) => legend.push({ color: PLANE_STROKES[i % 3], text: `平面${'①②③④⑤⑥'.charAt(i)} 方程${i + 1}` }));
  }
  if (path.length > 1) legend.push({ color: '#f5c542', text: '迭代轨迹 x⁽ᵏ⁾' });
  if (o.point) legend.push({ color: '#ffe08a', text: '解（三平面交点）' });
  ctx.font = '11px sans-serif';
  ctx.textAlign = 'left';
  let ly = 16;
  legend.forEach((it) => {
    ctx.fillStyle = 'rgba(12,20,36,0.55)';
    ctx.fillRect(mL + 4, ly - 10, 132, 15);
    ctx.strokeStyle = it.color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(mL + 10, ly - 3);
    ctx.lineTo(mL + 30, ly - 3);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255,255,255,0.9)';
    ctx.fillText(it.text, mL + 36, ly);
    ly += 16;
  });
}
