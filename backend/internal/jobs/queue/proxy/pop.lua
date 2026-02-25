local queue_heads_key = KEYS[1]
local current_time = tonumber(ARGV[1])
local lease_milliseconds = tonumber(ARGV[2])
local proxy_key_prefix = ARGV[3]

local function refresh_head(queue_key)
  local current = redis.call('ZRANGE', queue_key, 0, 0, 'WITHSCORES')
  if #current == 0 then
    redis.call('ZREM', queue_heads_key, queue_key)
    return nil
  end

  local score = tonumber(current[2])
  redis.call('ZADD', queue_heads_key, score, queue_key)
  return score
end

for _ = 1, 8 do
  local head = redis.call('ZRANGE', queue_heads_key, 0, 0, 'WITHSCORES')
  if #head == 0 then
    return {0, "", "", -1, -1}
  end

  local queue_key = head[1]
  local indexed_score = tonumber(head[2])
  local entry = redis.call('ZRANGE', queue_key, 0, 0, 'WITHSCORES')

  if #entry == 0 then
    redis.call('ZREM', queue_heads_key, queue_key)
  else
    local member = entry[1]
    local score = tonumber(entry[2])

    if score ~= indexed_score then
      redis.call('ZADD', queue_heads_key, score, queue_key)
    elseif score > current_time then
      return {0, "", "", score, -1}
    else
      local proxy_key = proxy_key_prefix .. member
      local proxy_data = redis.call('GET', proxy_key)

      if not proxy_data then
        redis.call('ZREM', queue_key, member)
        refresh_head(queue_key)
      else
        local lease_score = current_time + lease_milliseconds
        redis.call('ZADD', queue_key, lease_score, member)
        refresh_head(queue_key)
        return {1, member, proxy_data, score, -1}
      end
    end
  end
end

return {0, "", "", -1, -1}
