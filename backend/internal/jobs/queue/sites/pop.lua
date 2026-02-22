local result = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
if #result == 0 then return nil end

local member = result[1]
local score = tonumber(result[2])
local now = tonumber(ARGV[1])
local lease_seconds = tonumber(ARGV[2])

if score > now then return nil end

local site_key = KEYS[2] .. member
local site_data = redis.call('GET', site_key)
if not site_data then
  redis.call('ZREM', KEYS[1], member)
  return nil
end

redis.call('ZADD', KEYS[1], now + lease_seconds, member)

return {member, site_data, score}
