-- KEYS[1]: 库存的 key (如 seckill:stock:1001)
-- KEYS[2]: 已购买用户的 Set key (如 seckill:users:1001)
-- ARGV[1]: 用户ID
-- ARGV[2]: 购买数量 (如 1)

local stock = tonumber(redis.call('get', KEYS[1]))
if stock == nil then
    return -1 -- 库存不存在
end

-- 检查该用户是否已经在 Set 集合中（是否已经买过）
local isBought = redis.call('sismember', KEYS[2], ARGV[1])
if isBought == 1 then
    return -2 -- 代表重复购买
end

local buyNum = tonumber(ARGV[2])
if stock >= buyNum then
    -- 库存充足，扣减库存
    redis.call('decrby', KEYS[1], buyNum)
    -- 把该用户加入已购买名单
    redis.call('sadd', KEYS[2], ARGV[1])
    return 1 -- 扣减成功
else
    return 0 -- 库存不足
end