# OpenAI / Anthropic pricing snapshot, 2026-09-24

The adjacent JSON records official USD / million Token prices for the 25 enabled
model names on this installation. It is a reviewed configuration snapshot, not
an automatic startup override. Future edits remain in the pricing admin UI.
Group multipliers, model routing, global per-image prices and fixed image prices
are preserved. Existing requests keep their frozen pricing snapshot.

## Sources and scope

- https://developers.openai.com/api/docs/pricing
- https://developers.openai.com/api/docs/guides/fast-mode
- https://platform.claude.com/docs/en/about-claude/pricing
- https://platform.claude.com/docs/en/build-with-claude/fast-mode

OpenAI text models use explicit Standard, Fast/priority and Flex branches. The
272,000 input-token threshold uses total context length, including cache. Long
prices double input/cache/write rates and multiply output by 1.5. GPT-5.4 mini
has no published long tier. GPT-5.4 and GPT-5.5 have no published Fast long tier;
their Fast branches use the published flat Fast rates, with no invented long
surcharge. Upstream model/context support still applies. Batch is a separate
API and is not selected by a request-body field here.

`codex-auto-review` is priced as `gpt-5.6-sol`, as explicitly confirmed by the
administrator; this does not change channel model mappings. Sol's $4 input /
$20 output promotion is advertised through at least 2026-11-21; review after
that date. GPT-6 Luna is $0.10 input / $0.50 output / $0.01 cached input /
$0.125 cache write. Unsupported cache writes on GPT-5.4/5.5 are not separately
priced; the expression's ordinary input fallback remains.

Claude Fast is 2x on Opus 5.5, 5 and 4.8 only. Native Claude and converted
responses observe `usage.speed` (including SSE message_start/message_delta),
so an actual standard-speed response settles at standard rates. Missing speed
retains the final outgoing request's setting. The upstream fast beta header is
still required where the provider requires it. US inference geography is 1.1x
for Claude 4.6 and later; cache TTLs have distinct 5-minute and 1-hour prices.
Claude 4.5 and Haiku 4.5 retain their published standard rates. No unsupported
Fast or geographic surcharge is applied to them.

Image Token prices apply only to the enabled `OpenAI官key` image group. All three
image models have text input $5, cached text $1.25, image input $8, cached image
$2, image output $30. Text output and cache creation are unpublished/unsupported
lanes, represented as zero in the seven-field editor, not inferred prices for
a supported capability. Image billing priority and other groups are unchanged.

Prices determine the site's charge, multiplied by its effective group ratio.
Pass-through and overrides still determine the actual request sent upstream;
they can change upstream behavior and cost. Settlement uses confirmed upstream
service_tier / speed when available, and never changes the response body.

Configuration: Billing settings → model prices → billing expression; image
prices: Billing settings → group pricing → OpenAI官key → Token prices.
The JSON uses actual option keys `billing_setting.billing_expr` and
`billing_setting.billing_mode`, alongside ratio fallback maps and
`ImageTokenGroupPrices`. Unsupported models are not added by this snapshot.
