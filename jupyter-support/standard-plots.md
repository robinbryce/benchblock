---
jupyter:
  jupytext:
    text_representation:
      extension: .md
      format_name: markdown
      format_version: '1.3'
      jupytext_version: 1.11.4
  kernelspec:
    display_name: Python 3 (ipykernel)
    language: python
    name: python3
---

```python tags=["parameters"]
dbfile = "blocks.db"
plot_prefix = "raft7"
firstblock = None
lastblock = 5000
```

```python

```

# Summary

```python tags=[hide-output, show-input]
import bbplots.db as db
import bbplots.plots as plots
import matplotlib

blocks = db.Blocks(dbfile)
for k, v in blocks.common_stats(firstblock=firstblock, lastblock=lastblock).items():
    print(f"{k}={v}")
```

```python

matplotlib.rcParams["agg.path.chunksize"] = 10000

plots.GAS(blocks, plot_prefix=plot_prefix).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-gas.png",
    logy=False, firstblock=firstblock, lastblock=lastblock
    )

plots.BSZ(blocks, plot_prefix=plot_prefix).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-bsz.png",
    firstblock=firstblock, lastblock=lastblock
    )


plots.BT(blocks, plot_prefix=plot_prefix).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-bt.png",
    logy=False, firstblock=firstblock, lastblock=lastblock
    )

plots.TPS(blocks, plot_prefix=plot_prefix).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-tps.png",
    firstblock=firstblock, lastblock=lastblock
    )

```
