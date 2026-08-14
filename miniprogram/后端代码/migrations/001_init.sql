-- 阶段 0 数据库初始化（用 root 在 MySQL Workbench / mysql 命令行执行一次即可）

CREATE DATABASE IF NOT EXISTS physics_lab DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 给后端的专用账号（不要用 root 连业务库）
CREATE USER IF NOT EXISTS 'physics'@'%' IDENTIFIED BY 'physics123';
GRANT ALL PRIVILEGES ON physics_lab.* TO 'physics'@'%';
FLUSH PRIVILEGES;

USE physics_lab;

-- 文档 8.3：users 表（也可以不手动建，服务启动时 AutoMigrate 会自动建）
CREATE TABLE IF NOT EXISTS users (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  openid      VARCHAR(64) UNIQUE NOT NULL,
  role        ENUM('student','teacher','admin') NOT NULL DEFAULT 'student',
  name        VARCHAR(64),
  student_no  VARCHAR(32),
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_role (role)
);

INSERT INTO users (openid, role, name, student_no)
VALUES ('oDEV_FAKE_OPENID_001', 'student', '测试同学', '2023001')
ON DUPLICATE KEY UPDATE openid = openid;
