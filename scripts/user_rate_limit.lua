-- 用户限流Lua脚本
-- KEYS[1]: 限流key
-- ARGV[1]: 限制次数
-- ARGV[2]: 过期时间(秒)
-- 返回: 1-未超过限制, 0-超过限制
local current = redis.call('GET', KEYS[1])
if current and tonumber(current) >= tonumber(ARGV[1]) then
    return 0  -- 超过限制
else
    redis.call('INCR', KEYS[1])
    if tonumber(redis.call('GET', KEYS[1])) == 1 then
        redis.call('EXPIRE', KEYS[1], ARGV[2])  -- 第一次设置时设置过期时间
    end
    return 1  -- 未超过限制
end
