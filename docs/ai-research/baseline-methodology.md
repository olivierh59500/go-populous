# Campaign baseline research

This is a diagnostic run of the unchanged strategic AI in `internal/populous/advanced_ai.go`, against the unchanged historical computer, on every one of the 495 original campaign worlds (`Level.Number` 0–494).

- Advanced side: God (blue, player 0). Historical side: Devil (red, player 1).
- World initialization: `GenerateWorldWithRules(level, DecodeTerrainRules(landN))`, using original `level.dat`, the exact original terrain economics, normal initial resources, and the normal deterministic world RNG.
- Each simulation tick calls `TickWithAdvancedComputer(GodPlayer)` exactly once. Limit: 9,600 ticks = 20 minutes at 8 ticks/second.
- `win` and `loss` require actual elimination of all living non-ruin followers on one side. `draw` means simultaneous elimination; `ongoing` means both still alive at the 20-minute cutoff. Population advantage never counts as a win.
- The tests neither amend level restrictions nor grant mana, population, powers, land, moves, or extra action slots.
- `baseline.jsonl` streams completed worlds in completion order. `baseline.json` and `baseline.csv` sort by world number after all worlds finish.

## Metric caveats

`actions` counts only a `Computer.DoneTurn` marker still observable after a complete simulation tick. The engine resets this field at reaction-slot boundaries, so this is a **lower bound, not an exact action count**. In particular, reaction speed 1 resets the marker every tick, yielding zero observed markers. Do not use this field to compare action rates between difficulty profiles.

`offensive_sounds_both_sides`, `first_offense_tick`, and `last_offense_tick` aggregate both AIs. Included events are earthquake, volcano, flood, knight creation, and Armageddon. Swamp sound cannot reliably identify a cast rather than a triggered swamp and is therefore omitted.

`castles` and `towns` are the engine's computer statistics, refreshed immediately before the last tick's actions; destruction/construction during that final tick may change the exact number. `peak_castles` is the peak of that same sampled statistic.

`population` uses the game's standard population helper; `living_population` explicitly excludes ruins. `living_slots` likewise excludes ruins and population-zero followers. `slots` is allocated peep slice length, not live entity count. `dead_slots` contains zero-population entries; `ruin_slots` contains positive-population ruins.

## Source fingerprint

SHA-256 at audit launch:

```
df94a14826c717d53652d12b498555cb97a1768a1f573281d6d3390b37a1d382  internal/populous/advanced_ai.go
431c925edac21be3c131043aa2d09c0b4ed1bbb5397e2ecb7436689c1ba3ea28  internal/populous/world.go
389eddc565c25d0db48e763471cefb2fd06401cc13fa768f40a4b3912dc59401  assets/amiga/level.dat
4aafd9834d181e59359ec77a7ec01fb5a107738ae2a815e929641423c9c7bd19  assets/amiga/land0
d1232e66f77a74b06980829cb72f428c1920d1cef67b0d846c66eee803783d43  assets/amiga/land1
593897ae7c11450ade6e4bdb94ac7a812c8702528d7fb2e5b2f32eba71f0071b  assets/amiga/land2
a1c75d5248ed524c4a17e6f1168ab519a0537130bd7dc2052181552584fbe2f5  assets/amiga/land3
```
