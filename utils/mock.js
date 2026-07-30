// utils/mock.js - 无后端时的演示数据，上线前删除即可

export const MOCK_LEVELS = [
  {
    id: 1,
    name: '牛顿环实验',
    experiment_code: 'newton_ring',
    status: 'unlocked',
    difficulty: 2,
    best_score: 88
  },
  {
    id: 2,
    name: '示波器实验',
    experiment_code: 'oscilloscope',
    status: 'unlocked',
    difficulty: 3,
    best_score: 0
  },
  {
    id: 3,
    name: '单摆实验',
    experiment_code: 'pendulum',
    status: 'unlocked',
    difficulty: 1,
    best_score: 0
  }
];

export const MOCK_PROGRESS = {
  total: 3,
  passed: 1,
  best_score: 88,
  avg_score: 88,
  records: [
    { id: 1, level_name: '牛顿环实验', status: 'passed', score: 88, best_score: 88, attempts: 3 },
    { id: 2, level_name: '示波器实验', status: 'in_progress', score: 0, best_score: 0, attempts: 1 },
    { id: 3, level_name: '单摆实验', status: 'unlocked', score: 0, best_score: 0, attempts: 0 }
  ]
};

export const MOCK_CLASSES = [
  { id: 1, name: '物理实验1班', teacher_name: '张老师', member_count: 32 },
  { id: 2, name: '物理实验2班', teacher_name: '李老师', member_count: 28 }
];

export const MOCK_CLASS_STATS = {
  summary: { avg_score: 78.5, pass_rate: 0.72 },
  rows: [
    { user_id: 1, name: '测试同学', student_no: '20240001', best_score: 88, attempts: 3, passed: true },
    { user_id: 2, name: '小明', student_no: '20240002', best_score: 65, attempts: 5, passed: true },
    { user_id: 3, name: '小红', student_no: '20240003', best_score: 42, attempts: 2, passed: false }
  ]
};