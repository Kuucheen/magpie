local result = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
if #result == 0 then return nil end

local member = result[1]
local score = tonumber(result[2])
local current_time = tonumber(ARGV[1])
local lease_seconds = tonumber(ARGV[2])

if score > current_time then return nil end

local proxy_key = KEYS[2] .. member
local proxy_data = redis.call('GET', proxy_key)

if not proxy_data then
  redis.call('ZREM', KEYS[1], member)
  return nil
end

redis.call('ZADD', KEYS[1], current_time + lease_seconds, member)

return {member, proxy_data, score}
