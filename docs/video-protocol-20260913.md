# Connected video protocol and billing rules

The 45 models connected to channel 151 use the reviewed catalog in
common/video_model_contracts.json. The catalog comes from
https://api.aicopy.top/api/pricing (2026-09-13); its 24 per-second and 21 per-call
units govern both pricing display and task pre-consumption. Model prices and
group ratios remain configured site values. The captured public metadata is in
common/testdata/aicopy_pricing_20260913.json.

https://api.aione.help/docs/api/video-plugin-api.md documents one public
POST /v1/videos API for all video models, including those whose pricing metadata
still says openai. Chat/Responses requests for these video models are rejected
locally with the correct endpoint. Channel text tests direct the operator to
the studio.

AICopy receives JSON containing prompt, seconds, and extra (aspect_ratio,
resolution, reference arrays with roles). size is optional and represents pixel
dimensions; a resolution string such as 720p belongs in extra.resolution.
Studio files are uploaded once to /v1/uploads, with their MIME type preserved,
and replaced with HTTP(S) URLs. A first/last pair uses reference_images roles,
without a duplicate input_reference. Existing Sora and other host adapters keep
their protocols.

Video creation, including remix, has zero automatic retries regardless of the
global text retry setting. Existing-task polling continues. Local pre-consume
failures remain local and do not create misleading channel failure rows.
Video failure diagnostics record only selected parameter values and reference
counts, excluding prompts, URLs, names and file content.

Historical channel 151 errors on 2026-09-13 comprised 27 requests and 135 rows:
5 local pre-consume failures repeated 10 times, 5 Responses 404 requests repeated
10 times, 2 chat 404 requests repeated 10 times, and 15 video parameter failures.
A blank-input diagnostic reproduced the upstream Responses 404. The retained
upstream message for historical Grok 404 is generic, so its deeper provider
cause is not recoverable from those logs. No historical charges are changed.


For per-second models with no duration, the relay makes the first supported
duration explicit before both billing and submission. Model-fixed modes are
inferred for public clients that omit the studio-only mode field.
