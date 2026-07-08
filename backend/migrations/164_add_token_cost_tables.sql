CREATE TABLE IF NOT EXISTS token_cost_state (
    id SMALLINT PRIMARY KEY DEFAULT 1,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at_text TEXT NOT NULL DEFAULT '',
    rank_mode VARCHAR(32) NOT NULL DEFAULT 'plus',
    personal_recharge_r DOUBLE PRECISION NOT NULL DEFAULT 400,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT token_cost_state_singleton CHECK (id = 1)
);

CREATE TABLE IF NOT EXISTS token_cost_platforms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    balance_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    calc_balance_usd DOUBLE PRECISION,
    rate_r DOUBLE PRECISION NOT NULL DEFAULT 1,
    rate_usd DOUBLE PRECISION NOT NULL DEFAULT 1,
    plus DOUBLE PRECISION,
    pro_min DOUBLE PRECISION,
    pro_max DOUBLE PRECISION,
    note TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS token_cost_history (
    id BIGSERIAL PRIMARY KEY,
    sort_order INTEGER NOT NULL DEFAULT 0,
    at_text TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    total_balance DOUBLE PRECISION NOT NULL DEFAULT 0,
    pro_min DOUBLE PRECISION,
    pro_max DOUBLE PRECISION,
    plus DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS token_cost_events (
    id BIGSERIAL PRIMARY KEY,
    sort_order INTEGER NOT NULL DEFAULT 0,
    at_text TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_token_cost_platforms_sort
    ON token_cost_platforms(sort_order, id);

CREATE INDEX IF NOT EXISTS idx_token_cost_history_sort
    ON token_cost_history(sort_order, id);

CREATE INDEX IF NOT EXISTS idx_token_cost_events_sort
    ON token_cost_events(sort_order, id);

DROP TABLE IF EXISTS pg_temp.token_cost_seed_payload;
CREATE TEMP TABLE token_cost_seed_payload (data JSONB);

INSERT INTO token_cost_seed_payload (data) VALUES ($$
{
  "version": 1,
  "updatedAt": "2026-07-06T03:38:28.869Z",
  "rankMode": "plus",
  "personalRechargeR": 400,
  "platforms": [
    {"id":"torchai","name":"torchai","balanceUsd":180,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":null,"proMin":0.3,"proMax":null,"note":"当前 pro 口径，已涨到 0.3"},
    {"id":"okcodex","name":"okcodex","balanceUsd":58.82,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.1,"proMin":0.21,"proMax":null,"note":"余额 58.82$，plus 0.1 / pro 0.21"},
    {"id":"encore","name":"encore","balanceUsd":1055,"calcBalanceUsd":null,"rateR":1,"rateUsd":10,"plus":1,"proMin":1.69,"proMax":null,"note":"encore plus=1 / pro=1.69"},
    {"id":"qingflow","name":"qingflow","balanceUsd":12,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.09,"proMin":0.11,"proMax":null,"note":"余额已降到 12$"},
    {"id":"devpool","name":"devpool","balanceUsd":40,"calcBalanceUsd":null,"rateR":1,"rateUsd":10,"plus":null,"proMin":1.5,"proMax":null,"note":"余额已调整到 40$"},
    {"id":"zz1cc","name":"zz1cc","balanceUsd":397,"calcBalanceUsd":null,"rateR":36,"rateUsd":550,"plus":null,"proMin":2.8,"proMax":null,"note":"涨价到 2.8 倍"},
    {"id":"aisz","name":"aisz","balanceUsd":64,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.1,"proMin":0.2,"proMax":null,"note":"plus/pro 双池"},
    {"id":"dawclaude","name":"dawclaude","balanceUsd":161,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.3,"proMin":0.35,"proMax":0.5,"note":"dawcode/dawclaude 余额涨到 161$，pro 为区间倍率"},
    {"id":"eirouter","name":"eirouter","balanceUsd":60,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":null,"proMin":0.4,"proMax":null,"note":""},
    {"id":"tuling","name":"tuling","balanceUsd":25.4,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.35,"proMin":0.45,"proMax":null,"note":"新增平台"},
    {"id":"qianxing","name":"乾行","balanceUsd":125,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.12,"proMin":0.25,"proMax":null,"note":"余额涨到 125$；plus 0.12 / pro 0.25"},
    {"id":"5yuan","name":"5yuan","balanceUsd":10.91,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.08,"proMin":0.16,"proMax":null,"note":""},
    {"id":"xiaobai-code","name":"小白code","balanceUsd":20,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.1,"proMin":0.18,"proMax":null,"note":"余额涨到 20$"},
    {"id":"mikuapi","name":"mikuapi","balanceUsd":14,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.3,"proMin":0.5,"proMax":null,"note":"余额降到 14$"},
    {"id":"superapi","name":"superapi","balanceUsd":36,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.08,"proMin":0.22,"proMax":null,"note":"余额涨到 36$"},
    {"id":"tokeness","name":"tokeness","balanceUsd":75,"calcBalanceUsd":10.7142857143,"rateR":1,"rateUsd":1,"plus":0.03,"proMin":0.1,"proMax":null,"note":"75$ 原始余额；5m 输入或 30m 输出折算为 10.71$ 后计算购买力"},
    {"id":"词元","name":"词元","balanceUsd":285,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.2,"proMin":null,"proMax":null,"note":"余额涨到 285$"},
    {"id":"qiutian","name":"qiutian","balanceUsd":20,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.04,"proMin":null,"proMax":null,"note":""},
    {"id":"openhh","name":"openhh","balanceUsd":20,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.03,"proMin":null,"proMax":null,"note":""},
    {"id":"登仙赞助","name":"登仙赞助","balanceUsd":10,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.06,"proMin":0.15,"proMax":null,"note":""},
    {"id":"jucodex","name":"jucodex","balanceUsd":30,"calcBalanceUsd":null,"rateR":1,"rateUsd":1,"plus":0.15,"proMin":0.25,"proMax":null,"note":""}
  ],
  "history": [
    {"at":"2026-06-24","summary":"初始核算","totalBalance":2758.64,"proMin":3600.27,"proMax":3600.27,"plus":null},
    {"at":"2026-06-24","summary":"torchai 调到 0.2 倍率","totalBalance":2758.64,"proMin":3985.98,"proMax":3985.98,"plus":null},
    {"at":"2026-06-25","summary":"zz1cc 调到 2.8 倍率，torchai 恢复 0.35 倍率","totalBalance":2758.64,"proMin":3493.93,"proMax":3493.93,"plus":null},
    {"at":"2026-06-26","summary":"encore/okcodex 涨倍率，qingflow 余额调整，foyeapi/funny 用完，eirouter/aisz 余额更新，乾行到 24.71$","totalBalance":2755.45,"proMin":2874.02,"proMax":2874.02,"plus":null},
    {"at":"2026-06-26","summary":"aisz 调到 0.2 倍率，5yuan 调到 0.16 倍率","totalBalance":2755.45,"proMin":2803.93,"proMax":2803.93,"plus":null},
    {"at":"2026-06-26","summary":"乾行调到 0.16 倍率且余额到 38$，torchai 降到 0.28 倍率","totalBalance":2768.74,"proMin":2964.09,"proMax":2964.09,"plus":null},
    {"at":"2026-06-26","summary":"加入 plus/pro 双池口径，okcodex/qingflow/aisz 区分 plus，乾行 45$，eirouter 60$，新增 tuling","totalBalance":2782.14,"proMin":3063.37,"proMax":3063.37,"plus":null},
    {"at":"2026-06-27","summary":"encore 改为 plus=1.15/pro=2.38，新增 superapi 与 tokeness，删除余额为 0 的平台展示","totalBalance":2810.14,"proMin":2852.44,"proMax":2852.44,"plus":null},
    {"at":"2026-06-27","summary":"tokeness 加入 plus/pro 口径，更新 dawcode、mikuapi、乾行、5yuan、小白 的 plus/pro 倍率","totalBalance":2885.14,"proMin":3408.39,"proMax":3413.1,"plus":null},
    {"at":"2026-06-27","summary":"tokeness 改为先按 5m/30m 折算 10.71$ 再套 plus/pro 倍率","totalBalance":2885.14,"proMin":2765.53,"proMax":2770.24,"plus":3630.74},
    {"at":"2026-07-03","summary":"okcodex 余额调整为 58.82$，plus=0.1，pro=0.21","totalBalance":2179.96,"proMin":2790.96,"proMax":2795.67,"plus":3631.25},
    {"at":"2026-07-03","summary":"torchai pro 倍率涨到 0.3","totalBalance":2294.01,"proMin":2991.54,"proMax":3073.43,"plus":4030.52},
    {"at":"2026-7-3","summary":"手动更新","totalBalance":2574.01,"proMin":3127.54,"proMax":3209.43,"plus":7776.35},
    {"at":"2026-7-3","summary":"手动更新","totalBalance":2634.01,"proMin":3367.54,"proMax":3449.43,"plus":7126.35},
    {"at":"2026-07-04","summary":"encore plus=1/pro=1.69，devpool 余额 40$，词元余额 246$","totalBalance":2546.44,"proMin":3479.48,"proMax":3561.37,"plus":7343.96},
    {"at":"2026-07-05","summary":"dawcode 161$，乾行 125$，superapi 36$，小白 20$，mikuapi 14$，词元 285$，新增 composiastack 30$/0.3","totalBalance":2699.13,"proMin":3808.42,"proMax":3946.42,"plus":7969.8}
  ],
  "events": [
    {"at":"2026/6/29 22:12:03","title":"编辑平台","detail":"torchai 的 proMin：0.28 -> 0.25"},
    {"at":"2026/6/29 22:12:08","title":"编辑平台","detail":"torchai 的 proMin：0.25 -> 0.28"},
    {"at":"2026/6/29 22:12:11","title":"编辑平台","detail":"torchai 的 proMin：0.28 -> 0.25"},
    {"at":"2026/6/29 22:17:29","title":"编辑平台","detail":"dawclaude 的 balanceUsd：56 -> 95.54"},
    {"at":"2026/6/29 22:22:59","title":"编辑平台","detail":"tuling 的 balanceUsd：10.4 -> 25.4"},
    {"at":"2026/7/3 09:11:35","title":"编辑平台","detail":"okcodex：余额 764 -> 58.82，plus 1.3 -> 0.1，pro 3 -> 0.21"},
    {"at":"2026/7/3 12:11:12","title":"编辑平台","detail":"torchai 的 proMin：0.25 -> 0.3"},
    {"at":"2026/7/3 13:55:45","title":"编辑平台","detail":"乾行 的 plus：0.16 -> 0.12"},
    {"at":"2026/7/3 13:55:48","title":"编辑平台","detail":"乾行 的 proMin：0.3 -> 0.25"},
    {"at":"2026/7/3 13:56:25","title":"编辑平台","detail":"okcodex 的 rateR：7 -> 1"},
    {"at":"2026/7/3 13:56:28","title":"编辑平台","detail":"okcodex 的 rateUsd：100 -> 1"},
    {"at":"2026/7/3 14:00:03","title":"新增平台","detail":"词元，余额 230.00$，plus=0.1，pro=空"},
    {"at":"2026/7/3 14:00:29","title":"新增平台","detail":"qiutian，余额 20.00$，plus=0.04，pro=空"},
    {"at":"2026/7/3 14:00:55","title":"新增平台","detail":"openhh，余额 20.00$，plus=0.03，pro=空"},
    {"at":"2026/7/3 14:02:26","title":"新增平台","detail":"登仙赞助，余额 10.00$，plus=0.06，pro=0.1"},
    {"at":"2026/7/3 14:07:25","title":"记录历史点","detail":"2026-7-3 / 手动更新"},
    {"at":"2026/7/3 15:42:37","title":"编辑个人充值","detail":"个人充值总价：333r -> 400r"},
    {"at":"2026/7/3 15:43:23","title":"编辑平台","detail":"乾行 的 balanceUsd：54 -> 114"},
    {"at":"2026/7/3 21:06:13","title":"编辑平台","detail":"词元 的 plus：0.1 -> 0.2"},
    {"at":"2026/7/3 21:06:50","title":"记录历史点","detail":"2026-7-3 / 手动更新"},
    {"at":"2026/7/4 09:02:51","title":"编辑平台","detail":"encore 的 plus：1.15 -> 1，proMin：2.38 -> 1.69，note：encore plus/pro 双池 -> encore plus=1 / pro=1.69"},
    {"at":"2026/7/4 09:02:51","title":"编辑平台","detail":"devpool 的 balanceUsd：143.57 -> 40，note： -> 余额已调整到 40$"},
    {"at":"2026/7/4 09:02:51","title":"编辑平台","detail":"词元 的 balanceUsd：230 -> 246，note： -> 余额涨到 246$"},
    {"at":"2026/7/4 09:02:51","title":"记录历史点","detail":"2026-07-04 / encore plus=1/pro=1.69，devpool 余额 40$，词元余额 246$"},
    {"at":"2026/7/5 11:32:40","title":"编辑平台","detail":"dawclaude 的 balanceUsd：95.54 -> 161，note：pro 为区间倍率 -> dawcode/dawclaude 余额涨到 161$，pro 为区间倍率"},
    {"at":"2026/7/5 11:32:40","title":"编辑平台","detail":"乾行 的 balanceUsd：114 -> 125，note：plus 0.16 / pro 0.3 -> 余额涨到 125$；plus 0.12 / pro 0.25"},
    {"at":"2026/7/5 11:32:40","title":"编辑平台","detail":"superapi 的 balanceUsd：28 -> 36，note：新增平台 -> 余额涨到 36$"},
    {"at":"2026/7/5 11:32:40","title":"编辑平台","detail":"小白code 的 balanceUsd：14.6 -> 20，note：空 -> 余额涨到 20$"},
    {"at":"2026/7/5 11:32:40","title":"编辑平台","detail":"mikuapi 的 balanceUsd：20.17 -> 14，note：空 -> 余额降到 14$"},
    {"at":"2026/7/5 11:32:40","title":"编辑平台","detail":"词元 的 balanceUsd：246 -> 285，note：余额涨到 246$ -> 余额涨到 285$"},
    {"at":"2026/7/5 11:32:40","title":"新增平台","detail":"composiastack，余额 30.00$，plus=空，pro=0.3"},
    {"at":"2026/7/5 11:32:40","title":"记录历史点","detail":"2026-07-05 / dawcode 161$，乾行 125$，superapi 36$，小白 20$，mikuapi 14$，词元 285$，新增 composiastack 30$/0.3"},
    {"at":"2026/7/5 22:14:54","title":"新增平台","detail":"jucodex，余额 60.00$，plus=0.15，pro=0.25"},
    {"at":"2026/7/5 22:15:00","title":"编辑平台","detail":"jucodex 的 balanceUsd：60 -> 30"},
    {"at":"2026/7/6 10:58:06","title":"编辑平台","detail":"登仙赞助 的 proMin：0.1 -> 0.15"},
    {"at":"2026/7/6 11:08:58","title":"删除平台","detail":"composiastack 已从动态表移除"},
    {"at":"2026/7/6 11:38:28","title":"编辑平台","detail":"小白code 的 plus：0.13 -> 0.1"}
  ]
}
$$::JSONB);

INSERT INTO token_cost_state (id, version, updated_at_text, rank_mode, personal_recharge_r)
SELECT 1,
       COALESCE((data->>'version')::INTEGER, 1),
       COALESCE(data->>'updatedAt', ''),
       COALESCE(data->>'rankMode', 'plus'),
       COALESCE((data->>'personalRechargeR')::DOUBLE PRECISION, 400)
FROM token_cost_seed_payload
WHERE NOT EXISTS (SELECT 1 FROM token_cost_state WHERE id = 1);

INSERT INTO token_cost_platforms (
    id, name, balance_usd, calc_balance_usd, rate_r, rate_usd, plus, pro_min, pro_max, note, sort_order
)
SELECT item.value->>'id',
       item.value->>'name',
       COALESCE((item.value->>'balanceUsd')::DOUBLE PRECISION, 0),
       (item.value->>'calcBalanceUsd')::DOUBLE PRECISION,
       COALESCE((item.value->>'rateR')::DOUBLE PRECISION, 1),
       COALESCE((item.value->>'rateUsd')::DOUBLE PRECISION, 1),
       (item.value->>'plus')::DOUBLE PRECISION,
       (item.value->>'proMin')::DOUBLE PRECISION,
       (item.value->>'proMax')::DOUBLE PRECISION,
       COALESCE(item.value->>'note', ''),
       item.ordinality - 1
FROM token_cost_seed_payload
CROSS JOIN LATERAL jsonb_array_elements(data->'platforms') WITH ORDINALITY AS item(value, ordinality)
WHERE NOT EXISTS (SELECT 1 FROM token_cost_platforms);

INSERT INTO token_cost_history (sort_order, at_text, summary, total_balance, pro_min, pro_max, plus)
SELECT item.ordinality - 1,
       COALESCE(item.value->>'at', ''),
       COALESCE(item.value->>'summary', ''),
       COALESCE((item.value->>'totalBalance')::DOUBLE PRECISION, 0),
       (item.value->>'proMin')::DOUBLE PRECISION,
       (item.value->>'proMax')::DOUBLE PRECISION,
       (item.value->>'plus')::DOUBLE PRECISION
FROM token_cost_seed_payload
CROSS JOIN LATERAL jsonb_array_elements(data->'history') WITH ORDINALITY AS item(value, ordinality)
WHERE NOT EXISTS (SELECT 1 FROM token_cost_history);

INSERT INTO token_cost_events (sort_order, at_text, title, detail)
SELECT item.ordinality - 1,
       COALESCE(item.value->>'at', ''),
       COALESCE(item.value->>'title', ''),
       COALESCE(item.value->>'detail', '')
FROM token_cost_seed_payload
CROSS JOIN LATERAL jsonb_array_elements(data->'events') WITH ORDINALITY AS item(value, ordinality)
WHERE NOT EXISTS (SELECT 1 FROM token_cost_events);

DROP TABLE token_cost_seed_payload;
