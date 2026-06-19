-- 仅用于已经使用旧版代码写入过空邮箱的数据库。
-- 邮箱是可选字段；空字符串会触发唯一索引冲突，应统一归一为 NULL。

UPDATE users
SET email = NULL
WHERE email = '';
