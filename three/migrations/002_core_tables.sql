-- 002_core_tables.sql — 核心业务表 DDL + 种子数据（数据组规范脚本）
-- 用 root 执行一次即可。幂等：可重复执行。
-- 与服务启动时的 AutoMigrate + Go seed 等价：SQL 先行手动建库，或 AutoMigrate 自动建表后由 seedAll 填数据。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。

USE physics_lab;

-- ========== 7 张表 DDL（与 GORM model 字段一一对应）==========

CREATE TABLE IF NOT EXISTS users (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  openid       VARCHAR(64) UNIQUE NOT NULL,
  role         ENUM('student','teacher','admin') NOT NULL DEFAULT 'student',
  name         VARCHAR(64) NULL,
  student_no   VARCHAR(32) NULL,
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_role (role)
);

CREATE TABLE IF NOT EXISTS classes (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  name         VARCHAR(64) NOT NULL,
  teacher_id   BIGINT NOT NULL,
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_teacher (teacher_id)
);

CREATE TABLE IF NOT EXISTS class_members (
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  class_id   BIGINT NOT NULL,
  user_id    BIGINT NOT NULL,
  joined_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY idx_class_user (class_id, user_id),
  INDEX idx_user (user_id)
);

CREATE TABLE IF NOT EXISTS experiments (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  code         VARCHAR(32) UNIQUE NOT NULL,        -- newton_ring / oscilloscope / pendulum
  name         VARCHAR(64) NOT NULL,
  render_mode  VARCHAR(32) NOT NULL DEFAULT 'mixed_3d_2d',
  config       JSON NULL,                           -- 随实验类型不同（前端读取）
  target       JSON NULL,                           -- 评分目标（scoring.go 解析，必含 pass_score）
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS levels (
  id               BIGINT PRIMARY KEY AUTO_INCREMENT,
  experiment_id    BIGINT NOT NULL,
  name             VARCHAR(64) NOT NULL,
  order_no         INT NOT NULL DEFAULT 0,          -- 1,2,3... 解锁顺序
  difficulty       INT NOT NULL DEFAULT 1,
  prereq_level_id  BIGINT NULL,                     -- NULL 表示首关
  created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_experiment (experiment_id),
  INDEX idx_order (order_no),
  INDEX idx_prereq (prereq_level_id)
);

CREATE TABLE IF NOT EXISTS user_progress (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id     BIGINT NOT NULL,
  level_id    BIGINT NOT NULL,
  best_score  INT NOT NULL DEFAULT 0,
  last_score  INT NOT NULL DEFAULT 0,
  attempts    INT NOT NULL DEFAULT 0,
  passed      TINYINT(1) NOT NULL DEFAULT 0,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY idx_user_level (user_id, level_id)
);

CREATE TABLE IF NOT EXISTS operation_logs (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id     BIGINT NOT NULL,
  action      VARCHAR(32) NOT NULL,                 -- login / submit / class.create / class.delete / class.rename / class.member.add / class.member.remove
  level_id    BIGINT NULL,
  score       INT NULL,
  detail      TEXT NULL,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user (user_id),
  INDEX idx_action (action),
  INDEX idx_level (level_id),
  INDEX idx_user_time (user_id, created_at),
  INDEX idx_action_time (action, created_at)
);

-- ========== 种子：dev 测试账号（与 dev_ 后门对齐，登录即已补全）==========
INSERT INTO users (openid, role, name, student_no) VALUES
  ('oDEV_STUDENT', 'student', '测试同学', '2023001'),
  ('oDEV_TEACHER', 'teacher', '张老师', NULL),
  ('oDEV_ADMIN',   'admin',   '管理员', NULL)
ON DUPLICATE KEY UPDATE role = VALUES(role), name = VALUES(name);

-- ========== 种子：3 个实验 ==========
-- target 必含 pass_score：scoring.go 用 score>=pass_score 判过关，缺失则恒过关。
-- oscilloscope 的 channels 键用大写 CH1/CH2，与前端 oscilloscope.js 提交的 channel 大小写一致。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (1, 'newton_ring', '牛顿环实验', 'mixed_3d_2d',
    JSON_OBJECT('wavelength_nm', 589.3, 'lens_radius_mm', 855, 'k_range', JSON_ARRAY(1,10), 'tolerance_mm', 0.02),
    JSON_OBJECT('method','least_squares_R','lens_radius_mm',855,'wavelength_nm',589.3,'pass_score',60)),
  (2, 'oscilloscope', '示波器实验', 'mixed_3d_2d',
    JSON_OBJECT('channels', JSON_OBJECT(
      'CH1', JSON_OBJECT('A',2.0,'f',50,'phi',0),
      'CH2', JSON_OBJECT('A',1.5,'f',50,'phi',0.7854))),
    JSON_OBJECT('method','param_match','channels', JSON_OBJECT(
      'CH1', JSON_OBJECT('A',2.0,'f',50),
      'CH2', JSON_OBJECT('A',1.5,'f',50)),'pass_score',60)),
  (3, 'pendulum', '单摆实验', 'mixed_3d_2d',
    JSON_OBJECT('length_m',1.0,'angle_deg',15,'gravity',9.8),
    JSON_OBJECT('method','gravity_fit','gravity',9.8,'length_m',1.0,'pass_score',60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 种子：3 个关卡串成解锁链 L1(无前置) -> L2(前置L1) -> L3(前置L2) ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (1, 1, '牛顿环实验', 1, 2, NULL),
  (2, 2, '示波器实验', 2, 3, 1),
  (3, 3, '单摆实验',   3, 1, 2)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);

-- ========== 种子：示范班级（dev 教师建班 + dev 学生入班）==========
INSERT INTO classes (name, teacher_id)
SELECT '物理实验1班', id FROM users WHERE openid = 'oDEV_TEACHER'
WHERE NOT EXISTS (SELECT 1 FROM classes WHERE name = '物理实验1班'
  AND teacher_id = (SELECT id FROM users WHERE openid = 'oDEV_TEACHER'));

INSERT INTO class_members (class_id, user_id, joined_at)
SELECT c.id, u.id, NOW()
FROM classes c JOIN users u
WHERE c.name = '物理实验1班' AND u.openid = 'oDEV_STUDENT'
  AND NOT EXISTS (SELECT 1 FROM class_members cm WHERE cm.class_id = c.id AND cm.user_id = u.id);
