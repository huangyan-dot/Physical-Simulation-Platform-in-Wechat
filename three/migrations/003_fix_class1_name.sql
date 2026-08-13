-- 一次性修复：2026-08-10 联调时用 Git Bash curl 直接发中文，GBK 字节入库导致 classes.id=1 名字乱码。
-- 执行：mysql -uphysics -pphysics123 --default-character-set=utf8mb4 physics_lab < migrations/003_fix_class1_name.sql
UPDATE classes SET name = '物理实验1班' WHERE id = 1;
