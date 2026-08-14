// utils/quiz.js - 实验自测题库
// 参考：张兆奎《大学物理实验（第四版）》
//
// 题型：single（单选）/ judge（判断）/ calc（计算，带容差）
// 每个实验从题库随机抽 QUIZ_SIZE 道题。

export const QUIZ_SIZE = 5;

const QUIZ_BANK = {
  newton_ring: [
    {
      type: 'single',
      question: '牛顿环干涉图样的中心是一个暗斑，其原因是',
      options: [
        '该处空气膜厚度为零，两束光光程差为零',
        '存在半波损失，产生 λ/2 的附加光程差',
        '透镜材料吸收了该处的光',
        '显微镜叉丝挡住了中心的光'
      ],
      answer: 1,
      hint: '想一想：光从空气射向玻璃表面反射时，相位会发生什么变化？',
      explanation:
        '中心接触处 d = 0，但光从光疏介质（空气）射向光密介质（玻璃）表面反射时会产生 λ/2 的附加光程差，' +
        '即半波损失。故总光程差 δ = 0 + λ/2 = λ/2，满足暗纹条件，中心为暗斑。'
    },
    {
      type: 'single',
      question: '牛顿环实验中采用测量两环直径平方差的方法，主要目的是',
      options: [
        '提高读数显微镜的分辨率',
        '消除因接触点变形、灰尘导致中心无法准确确定的系统误差',
        '减小钠光灯波长的不确定度',
        '消除鼓轮的回程差'
      ],
      answer: 1,
      hint: '直接用 r² = kλR 需要知道什么量？这个量容易准确测出吗？',
      explanation:
        '透镜与平板接触处受压变形且可能夹有灰尘，接触点不是理想几何点，中心位置难以准确确定。' +
        '用 Dₘ² − Dₙ² = 4(m−n)λR 计算时，公式与环级数绝对值和中心位置均无关，因而消除了这一系统误差。'
    },
    {
      type: 'single',
      question: '测量牛顿环直径时，鼓轮必须始终单向转动，这是为了避免',
      options: ['视差', '回程差（空程误差）', '半波损失', '色差'],
      answer: 1,
      hint: '丝杆与螺母之间存在间隙，反向转动时会发生什么？',
      explanation:
        '读数显微镜的丝杆与螺母间存在间隙，反向转动时鼓轮先转过一个角度而叉丝并不移动，' +
        '这一空程造成的误差称为回程差。单向转动可避免此误差。'
    },
    {
      type: 'judge',
      question: '牛顿环的干涉条纹是等间距分布的。',
      answer: false,
      hint: '由 r = √(kλR) 看，r 与 k 是什么关系？',
      explanation:
        '由 r = √(kλR)，环半径与级数的平方根成正比，故相邻环的间距随 k 增大而减小，' +
        '即条纹内疏外密，并非等间距。'
    },
    {
      type: 'judge',
      question: '牛顿环装置框上的三个螺丝应尽量旋紧，以使条纹更清晰。',
      answer: false,
      hint: '想想过度压紧会对透镜造成什么后果？',
      explanation:
        '螺丝只需轻轻旋紧到能看到位于中心的干涉环即可。旋得过紧会使透镜变形过大甚至压碎透镜，' +
        '同时也会增大接触形变带来的误差。'
    },
    {
      type: 'calc',
      question:
        '用钠光（λ = 589.3 nm）观察牛顿环，测得第 20 环直径 D₂₀ = 4.28 mm，' +
        '第 10 环直径 D₁₀ = 3.02 mm。求透镜曲率半径 R（单位 mm，保留整数）。',
      answer: 390,
      tolerance: 0.06,
      unit: 'mm',
      hint: '用平方差公式 R = (Dₘ² − Dₙ²) / [4(m−n)λ]，注意把 λ 换成 mm。',
      explanation:
        'λ = 589.3 nm = 5.893×10⁻⁴ mm\n' +
        'D₂₀² − D₁₀² = 4.28² − 3.02² = 18.318 − 9.120 = 9.198 mm²\n' +
        'R = 9.198 / (4 × 10 × 5.893×10⁻⁴) = 9.198 / 0.023572 ≈ 390 mm'
    },
    {
      type: 'calc',
      question:
        '已知平凸透镜曲率半径 R = 855 mm，用钠光（λ = 589.3 nm）照射，' +
        '求第 5 个暗环的半径 r₅（单位 mm，保留三位小数）。',
      answer: 1.587,
      tolerance: 0.03,
      unit: 'mm',
      hint: '用 r = √(kλR)，注意波长要换算成 mm。',
      explanation:
        'λ = 589.3 nm = 5.893×10⁻⁴ mm\n' +
        'r₅ = √(5 × 5.893×10⁻⁴ × 855) = √(2.5193) ≈ 1.587 mm'
    },
    {
      type: 'single',
      question: '若把牛顿环装置中的空气膜换成折射率 n = 1.33 的水膜，则干涉环会',
      options: ['半径变大，条纹变疏', '半径变小，条纹变密', '半径不变', '条纹消失'],
      answer: 1,
      hint: '介质中光程差为 2nd + λ/2，暗环条件变成什么？',
      explanation:
        '充水后光程差为 2nd + λ/2，暗环条件为 d = kλ/(2n)，' +
        '代入 r² = 2Rd 得 r = √(kλR/n)。因 n > 1，同一级环半径变小，条纹变密。'
    },
    {
      type: 'single',
      question: '观察牛顿环时，叉丝应当如何对准暗环？',
      options: [
        '叉丝与环相交，取交点',
        '叉丝与环相切，且每次取环的同一侧',
        '任意位置均可，不影响直径差',
        '叉丝对准环的最外缘'
      ],
      answer: 1,
      hint: '测量的是直径，两侧读数应当对应环的对称位置。',
      explanation:
        '应使叉丝与暗环相切，并且左右两侧都取暗环的同一侧（如都取暗纹中心），' +
        '这样两读数之差才是该环的直径。若两侧取法不一致会引入系统误差。'
    },
    {
      type: 'judge',
      question: '牛顿环实验中，改用波长更短的光源（如汞灯绿光）时，条纹会变密。',
      answer: true,
      hint: 'r = √(kλR) 中 λ 减小，r 如何变化？',
      explanation:
        '由 r = √(kλR)，λ 减小则同级环半径减小，相邻环间距也减小，故条纹变密。'
    },
    {
      type: 'single',
      question: '牛顿环属于哪一类干涉？',
      options: ['等倾干涉', '等厚干涉', '多光束干涉', '衍射现象'],
      answer: 1,
      hint: '干涉条纹的形状由什么量决定？',
      explanation:
        '牛顿环中，空气膜厚度相同的各点对应相同的干涉情况，条纹是厚度相等点的轨迹，' +
        '故属于等厚干涉。等倾干涉则是厚度均匀的薄膜在不同入射角下产生的干涉。'
    }
  ],

  oscilloscope: [
    {
      type: 'single',
      question: '示波器要显示稳定不移动的波形，扫描电压的周期必须',
      options: [
        '等于被测信号周期的整数倍',
        '等于被测信号周期的 1/2',
        '与被测信号周期无关',
        '远大于被测信号周期'
      ],
      answer: 0,
      hint: '每次扫描都要从波形的同一相位开始，才能重叠成稳定图像。',
      explanation:
        '只有 T扫描 = n·T信号（n 为正整数）时，每次扫描描出的波形才完全重合，' +
        '屏上显示 n 个完整且稳定的波形。这一条件称为同步（触发）条件。'
    },
    {
      type: 'single',
      question: '示波器 X 轴上加锯齿波扫描电压的作用是',
      options: [
        '使电子束加速',
        '使亮点在水平方向随时间匀速移动，把时间轴展开',
        '增强荧光屏亮度',
        '滤除信号中的噪声'
      ],
      answer: 1,
      hint: '锯齿波电压随时间线性增长，会让亮点如何运动？',
      explanation:
        '锯齿波电压随时间线性增加，使亮点在水平方向匀速移动，从而把时间作为横坐标展开。' +
        '与加在 Y 轴的被测信号配合，即可在屏上描绘出信号随时间变化的波形。'
    },
    {
      type: 'calc',
      question:
        '示波器 Y 轴灵敏度为 0.5 V/div，波形峰峰值占 4.0 格；' +
        '扫描速度 t/div = 2 ms/div，一个完整周期占 5.0 格。求信号频率 f（单位 Hz，保留整数）。',
      answer: 100,
      tolerance: 0.05,
      unit: 'Hz',
      hint: '先算周期 T = (t/div) × 格数，再取倒数。',
      explanation:
        'T = 2 ms/div × 5.0 div = 10 ms = 0.010 s\n' +
        'f = 1/T = 1/0.010 = 100 Hz\n' +
        '（顺带：U(p-p) = 0.5 V/div × 4.0 div = 2.0 V）'
    },
    {
      type: 'calc',
      question:
        '示波器 Y 轴灵敏度 0.2 V/div，测得正弦波峰峰值占 6.0 格。' +
        '求该信号的有效值（单位 V，保留三位小数）。',
      answer: 0.424,
      tolerance: 0.05,
      unit: 'V',
      hint: '先求峰峰值，再求峰值，正弦波有效值 = 峰值/√2。',
      explanation:
        'U(p-p) = 0.2 × 6.0 = 1.2 V\n' +
        '峰值 Uₘ = 1.2/2 = 0.6 V\n' +
        '有效值 U = Uₘ/√2 = 0.6/1.414 ≈ 0.424 V'
    },
    {
      type: 'single',
      question:
        '观察李萨如图形时，图形与水平线有 3 个切点，与竖直线有 2 个切点，' +
        '若 X 轴信号频率为 100 Hz，则 Y 轴信号频率为',
      options: ['150 Hz', '66.7 Hz', '200 Hz', '50 Hz'],
      answer: 0,
      hint: '公式为 f_Y / f_X = n(水平切点) / n(竖直切点)。',
      explanation:
        '设 x = sin(2πf_X t)，y = sin(2πf_Y t + φ)。在一个公共周期内，' +
        'x 达到极值（与竖直线相切）的次数为 f_X 的倍数，y 达到极值（与水平线相切）的次数为 f_Y 的倍数，\n' +
        '故　f_Y / f_X = n(水平切点) / n(竖直切点) = 3/2\n' +
        'f_Y = 100 × 3/2 = 150 Hz'
    },
    {
      type: 'judge',
      question: '使用示波器时，可以让亮点长时间静止停留在荧光屏的某一点上。',
      answer: false,
      hint: '想想电子束长时间轰击同一点会有什么后果？',
      explanation:
        '亮点长时间静止于一点会使该处荧光物质因持续受电子束轰击而烧损，形成永久暗斑。' +
        '应及时开启扫描或降低亮度。'
    },
    {
      type: 'judge',
      question: '测量信号周期时，测量多个周期的总时间再除以周期数，可以减小相对误差。',
      answer: true,
      hint: '读数误差是固定的，被测总量越大，相对误差如何变化？',
      explanation:
        '读数的绝对误差基本固定（约半格），测量 n 个周期总长度时相对误差被稀释为原来的 1/n，' +
        '故测多周期再平均可显著减小相对误差。这与单摆实验的累积法是同一思想。'
    },
    {
      type: 'single',
      question: '示波器面板上"聚焦"旋钮的作用是',
      options: [
        '调节波形的水平位置',
        '调节电子束的会聚程度，使光点细小清晰',
        '调节波形的竖直幅度',
        '调节扫描速度'
      ],
      answer: 1,
      hint: '聚焦影响的是光点的"粗细"还是"位置"？',
      explanation:
        '聚焦旋钮改变聚焦阳极的电位，调节电子透镜的会聚能力，使电子束在荧光屏上会聚成' +
        '细小清晰的光点，从而提高波形的清晰度和测量精度。'
    },
    {
      type: 'single',
      question: '若示波器上显示的波形在屏幕上左右不停移动，最可能的原因是',
      options: [
        '亮度调节不当',
        '未满足同步（触发）条件',
        '聚焦不良',
        'Y 轴灵敏度选择过大'
      ],
      answer: 1,
      hint: '波形移动说明每次扫描的起始相位在变化。',
      explanation:
        '波形左右移动说明扫描周期不是信号周期的整数倍，每次扫描起始相位不同，' +
        '导致波形不重合。应调节触发电平或扫描微调使之同步。'
    },
    {
      type: 'calc',
      question:
        '示波器扫描速度为 0.5 ms/div，屏上显示 2.5 个完整周期共占 10.0 格。' +
        '求信号频率 f（单位 Hz，保留整数）。',
      answer: 500,
      tolerance: 0.05,
      unit: 'Hz',
      hint: '先求 10 格对应的总时间，再除以周期数得 T。',
      explanation:
        '总时间 = 0.5 ms/div × 10.0 div = 5.0 ms\n' +
        'T = 5.0 ms / 2.5 = 2.0 ms = 0.002 s\n' +
        'f = 1/0.002 = 500 Hz'
    },
    {
      type: 'judge',
      question: '示波器只能测量电压信号，不能测量频率和相位。',
      answer: false,
      hint: '屏幕的横轴代表什么物理量？',
      explanation:
        '示波器的横轴是时间轴，因此不仅能测电压幅度，还能测周期、频率，' +
        '以及通过双踪显示或李萨如图形测量两信号的相位差。'
    }
  ],

  pendulum: [
    {
      type: 'single',
      question: '单摆做简谐运动的条件是',
      options: [
        '摆长足够长',
        '摆角足够小（一般小于 5°）',
        '摆球质量足够大',
        '悬线不可伸长'
      ],
      answer: 1,
      hint: '推导中把 sinθ 近似成了什么？这个近似何时成立？',
      explanation:
        '推导中用了 sinθ ≈ θ 的近似，只有摆角很小时才成立。' +
        '此时回复力与位移成正比，单摆才做简谐运动。实验中一般要求 θ < 5°。'
    },
    {
      type: 'single',
      question: '单摆实验中采用累积法（测 50 个周期总时间）的目的是',
      options: [
        '使摆动更稳定',
        '减小因人的反应时间引起的计时相对误差',
        '消除空气阻力的影响',
        '减小摆长的测量误差'
      ],
      answer: 1,
      hint: '人按秒表的反应误差约 0.2s，测 1 个周期和测 50 个周期，这个误差占比差多少？',
      explanation:
        '人的反应时间误差约 0.1~0.2 s 且基本固定。若只测 1 个周期（约 2 s），相对误差达 10%；' +
        '测 50 个周期（约 100 s），相对误差降到 0.2%。累积法把固定的绝对误差稀释了 50 倍。'
    },
    {
      type: 'single',
      question: '单摆计时的起止点应选在',
      options: [
        '摆球到达最高点时',
        '摆球通过平衡位置时',
        '任意位置均可',
        '摆球速度为零时'
      ],
      answer: 1,
      hint: '在哪个位置摆球速度最大，位置判断最准确？',
      explanation:
        '摆球通过平衡位置时速度最大，通过该位置的时刻最容易准确判断；' +
        '而在最高点速度趋于零，摆球在附近停留时间长，难以准确判定时刻，误差大。'
    },
    {
      type: 'calc',
      question:
        '测得摆长 L = 0.995 m，连续 50 个周期的总时间 t = 100.0 s。' +
        '求重力加速度 g（单位 m/s²，保留两位小数）。',
      answer: 9.82,
      tolerance: 0.02,
      unit: 'm/s²',
      hint: '先求 T = t/50，再用 g = 4π²L/T²。',
      explanation:
        'T = 100.0/50 = 2.000 s\n' +
        'g = 4π²L/T² = 4 × 9.8696 × 0.995 / 2.000² = 39.2795/4.000 ≈ 9.82 m/s²'
    },
    {
      type: 'calc',
      question:
        '悬线长 l = 98.50 cm，摆球直径 d = 2.00 cm。求摆长 L（单位 cm，保留两位小数）。',
      answer: 99.5,
      tolerance: 0.02,
      unit: 'cm',
      hint: '摆长是悬点到摆球质心的距离。',
      explanation:
        '摆长应为悬点到摆球质心（球心）的距离：\n' +
        'L = l + d/2 = 98.50 + 2.00/2 = 98.50 + 1.00 = 99.50 cm'
    },
    {
      type: 'judge',
      question: '单摆的周期与摆球的质量有关，质量越大周期越长。',
      answer: false,
      hint: '看周期公式 T = 2π√(L/g) 中有没有质量 m？',
      explanation:
        '由 T = 2π√(L/g) 可见，周期只与摆长和重力加速度有关，与摆球质量无关。' +
        '推导中回复力 F = −(mg/L)x 里的 m 与惯性 m 相消，故质量不影响周期。'
    },
    {
      type: 'judge',
      question: '摆角越大，单摆的周期越长。',
      answer: true,
      hint: '回忆大摆角时的修正公式。',
      explanation:
        '严格解为 T = 2π√(L/g)·[1 + (1/4)sin²(θ/2) + ...]，修正项随 θ 增大而增大，' +
        '故摆角越大周期越长。θ = 15° 时周期比小角度值大约 0.4%。'
    },
    {
      type: 'calc',
      question:
        '某地重力加速度 g = 9.80 m/s²，要使单摆周期恰好为 2.00 s，' +
        '摆长 L 应为多少（单位 m，保留三位小数）？',
      answer: 0.993,
      tolerance: 0.02,
      unit: 'm',
      hint: '由 T = 2π√(L/g) 反解 L。',
      explanation:
        '由 T = 2π√(L/g) 得 L = gT²/(4π²)\n' +
        'L = 9.80 × 4.00 / (4 × 9.8696) = 39.20/39.478 ≈ 0.993 m'
    },
    {
      type: 'single',
      question: '下列做法中，会使测得的 g 值偏小的是',
      options: [
        '把摆长误测得偏大',
        '把周期误测得偏大',
        '摆角控制得更小',
        '增大摆球质量'
      ],
      answer: 1,
      hint: '由 g = 4π²L/T²，看 T 偏大时 g 如何变化。',
      explanation:
        '由 g = 4π²L/T²，g 与 T² 成反比。若 T 测得偏大，则算出的 g 偏小。' +
        '（而 L 测得偏大会使 g 偏大；摆角和质量不影响或影响极小。）'
    },
    {
      type: 'single',
      question: '单摆实验中，若摆球在摆动过程中形成了圆锥摆，将导致',
      options: [
        '周期不受影响',
        '测量结果产生系统误差，应重新释放摆球',
        '周期一定变短',
        '摆长测量出错'
      ],
      answer: 1,
      hint: '圆锥摆还满足单摆的运动方程吗？',
      explanation:
        '圆锥摆不是在同一竖直平面内的简谐运动，其周期公式与单摆不同，' +
        '会引入系统误差。释放摆球时应避免给横向初速度，确保在同一竖直平面内摆动。'
    },
    {
      type: 'judge',
      question: '在地球上不同纬度处，用同一单摆测得的周期相同。',
      answer: false,
      hint: '重力加速度 g 在各处都一样吗？',
      explanation:
        '地球是椭球且自转，重力加速度随纬度增加而增大（赤道约 9.780，两极约 9.832 m/s²）。' +
        '由 T = 2π√(L/g)，g 不同则同一单摆周期不同，纬度越高周期略短。'
    }
  ]
};

/**
 * 从指定实验题库随机抽取 n 道题（Fisher–Yates 洗牌，不重复）。
 * 返回的题目对象带 __idx 字段记录在原题库中的位置，便于判分。
 */
export function pickQuiz(code, n = QUIZ_SIZE) {
  const bank = QUIZ_BANK[code];
  if (!bank || bank.length === 0) return [];

  const pool = bank.map((q, i) => ({ ...q, __idx: i }));
  // Fisher–Yates 洗牌
  for (let i = pool.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [pool[i], pool[j]] = [pool[j], pool[i]];
  }
  return pool.slice(0, Math.min(n, pool.length));
}

/**
 * 判定单题是否答对。
 * @param {Object} q 题目对象
 * @param {*} userAnswer 单选为选项下标，判断为 true/false，计算为数字字符串
 */
export function checkAnswer(q, userAnswer) {
  if (userAnswer === null || userAnswer === undefined || userAnswer === '') return false;

  if (q.type === 'single') {
    return Number(userAnswer) === q.answer;
  }
  if (q.type === 'judge') {
    return Boolean(userAnswer) === q.answer;
  }
  if (q.type === 'calc') {
    const v = parseFloat(userAnswer);
    if (!isFinite(v)) return false;
    const tol = q.tolerance || 0.05;
    return Math.abs(v - q.answer) <= Math.abs(q.answer) * tol;
  }
  return false;
}

export function getBankSize(code) {
  return (QUIZ_BANK[code] || []).length;
}
