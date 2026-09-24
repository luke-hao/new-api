# Fast service tier billing

## 生图计费优先级（保持原有顺序）

- 生图模型所在计费分组开启 Token：分组＋原始模型的生图 Token 单价，结算乘有效分组倍率。缺少该模型单价时报配置错误。
- 未开启 Token：用户分组＋计费分组＋模型＋尺寸的图片固定价 → 已启用的模型表达式 → 全局 ModelPrice 单张价 → 模型及输出等 Token 倍率。
- 图片固定价是绝对美元／张，命中后有效分组倍率为 1；没有命中才向后回退。全局单张价仍保留原来的尺寸／质量系数和张数处理。

## Fast 配置

在「计费设置 → 模型定价」编辑对应的原始模型，选择表达式计费。可在请求规则中选择参数 `service_tier`、等于 `fast`、填写自定倍率，或使用完整表达式分别填写输入、输出和缓存命中价格。单位美元／百万 Token；最终乘有效分组倍率。各模型独立设置，没有自动套用官方倍率。

以下仅为演示单价，不代表某个具体模型的官方报价：

```text
param("service_tier") in ["fast", "priority"]
  ? tier("fast", p * 4 + c * 20 + cr * 0.4)
  : tier("standard", p * 2 + c * 10 + cr * 0.2)
```

显式 `param("service_tier") == "fast"`、`== "priority"`、`in ["fast", "priority"]` 规则都可作为 Fast 定价配置。单独在名称／注释里写 fast 或设置基础价格不算已配置。可配置 1 倍，表示管理员明确允许同价。仅配置其中一个名称时，另一个在计费探针中使用等价名称，发给上游的 JSON 保持原值。

## 透传和结算

- 开启 service_tier 透传：保留请求的档位。
- 开启整个请求体透传：保留原请求体，包括档位；沿用原来的规则，跳过普通字段过滤及参数覆盖。
- 两者关闭：普通转换路径删除 service_tier；若后续参数覆盖再次添加，则按最终添加的值检查。
- 发出 Chat Completions／Responses 请求前，根据最终 JSON 和 Header 校验及预扣；每次重试刷新探针和分组倍率。最终请求为 fast／priority 且未配置对应价格时，返回 HTTP 400、model_price_error、提示“fast 价格未配置”，不发出该上游请求，已有预扣走原有退款流程。
- 上游响应的实际 service_tier 优先用于结算；default 降级按普通表达式分支，fast／priority 等价。未返回档位时按最终发出的请求结算。使用流式、非流式和 Chat→Responses 转换时均观察响应档位。
- OpenAI 项目默认档位也可能是 Fast，即使请求省略 service_tier。这样的档位只能在收到响应后获知；未配置 Fast 价格仍返回配置错误。若要避免发生这类上游成本，请把官方项目默认档位设为 Standard，或先补齐 Fast 价格。仅关闭本站透传不控制官方项目默认值。
- 使用日志 other 中保存 requested_service_tier 与 upstream_service_tier（上游有返回时），便于核对请求档位和降级结果。

官方资料（2026-09-24 核对）：
- https://developers.openai.com/api/docs/guides/priority-processing
- https://developers.openai.com/api/docs/pricing

本次只改变 Fast 价格校验和 OpenAI 请求条件计费依据，不变更生图优先级、现有价格选项和历史账单。
