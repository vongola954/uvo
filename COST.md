# UVO cost model (P0b)

> Fill AceData ₽ numbers from your platform invoice. Until then dual-output stays **OFF**.

## App credit costs (current)

| Op | Credits | Notes |
|----|--------:|-------|
| Generate (1 clip) | 1 | AceData keeps `Data[0]` only |
| Cover | 2 | |
| Karaoke / stems | 2 | instrumental + vocals + timing |
| Voice clone | 2 | |
| Edit | 1 | |
| TTS | 1 | |
| Portrait | 2–3 | Hedra/Kling |

## Packs (retail)

| Pack | Credits | Price ₽ | ₽/credit | ₽/song @1cr |
|------|--------:|--------:|---------:|------------:|
| free | 2 | 0 | — | 0 |
| pack5 (entry) | 5 | 99 | 19.8 | 99→~20 |
| pack10 | 10 | 199 | 19.9 | ~20 |
| pack30 | 30 | 499 | 16.6 | ~17 |
| pack100 | 100 | 699 | 7.0 | ~7 |
| pack500 | 500 | 1690 | 3.4 | ~3.4 |
| pack2000 | 2000 | 5990 | 3.0 | ~3 |

## Provider unit cost (fill in)

| Item | Value | Source |
|------|------:|--------|
| AceData 1× music gen (RUB) | **TBD** | platform.acedata.cloud invoice |
| AceData stems | **TBD** | |
| Target gross margin on pack100+ | ≥40% | plan 10/10 |

Formula:

```
margin = 1 - (acedata_rub_per_gen / rub_per_credit_sold)
```

Example: if AceData = 4 ₽/gen and pack100 sells at 7 ₽/credit → margin ≈ 43% → dual OK only if still ≥40% after 2× provider calls.

## Dual-output policy

| Condition | Policy |
|-----------|--------|
| AceData cost unknown / margin &lt; 40% | **Defer dual** — ship 1 clip / 1 credit |
| Margin ≥40% with 2 provider calls | Dual: 1 credit → 2 variants (or 2 credits — pick after numbers) |
| AceData returns 2 clips in one task | Prefer keep both without 2nd billable call |

**Current code:** single clip. Do not enable dual until this table is filled and margin gate passes.

## Ops checklist

1. Paste AceData ₽/gen into TBD rows.
2. Recompute margin for pack5 / pack100.
3. Go/no-go dual in `ROADMAP` / next epoch.
4. YooKassa keys (P0a) — separate; checkout already coded.
