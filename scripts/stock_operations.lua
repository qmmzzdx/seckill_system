-- scripts/stock_operations.lua
-- 原子性地检查并减少库存
local function check_and_decr_stock(key)
    local stock = redis.call('get', key)
    
    if not stock then
        return -1  -- key不存在
    end
    
    stock = tonumber(stock)
    if stock <= 0 then
        return -2  -- 库存不足
    end
    
    local new_stock = redis.call('decr', key)
    return new_stock
end

-- 原子性地检查并设置库存（如果库存不存在）
local function check_and_set_stock(key, new_stock)
    local existing = redis.call('exists', key)
    if existing == 0 then
        redis.call('set', key, new_stock)
        return 1  -- 设置成功
    else
        return 0  -- 库存已存在
    end
end

-- 主执行逻辑
local command = ARGV[1]
local key = KEYS[1]

if command == 'check_and_decr' then
    return check_and_decr_stock(key)
elseif command == 'check_and_set' then
    local new_stock = tonumber(ARGV[2])
    return check_and_set_stock(key, new_stock)
elseif command == 'get_stock' then
    local stock = redis.call('get', key)
    return stock or 0
else
    return -99  -- 未知命令
end
