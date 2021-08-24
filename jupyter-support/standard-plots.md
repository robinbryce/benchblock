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
plot_prefix = "raft"
timescale = 1000000000.0
firstblock = None
lastblock = None
```

# Summary

These charts are a reworking of those created for the chainhammer project.

> Ethereum benchmarking scripts "chainhammer" and "chainreader"
> by Dr Andreas Krueger, London 2018
> -- https://github.com/drandreaskrueger/chainhammer

```python tags=[hide-output, show-input]
import numpy
import matplotlib
import bbplots.db as db
import bbplots.plots as plots
from bbplots.blockframe import Frame

matplotlib.rcParams["agg.path.chunksize"] = 10000

blocks = db.Blocks(dbfile)
stats = blocks.common_stats(firstblock=firstblock, lastblock=lastblock)
for k, v in stats.items():
    print(f"{k}={v}")

f = Frame(blocks.load_frame(firstblock=firstblock, lastblock=lastblock)) 
df = f.df()
duration = (df["timestamp"][len(df["timestamp"])-1] - df["timestamp"][0])/ timescale
print(f"Sample duration    : {duration}")
implied_rate = stats["txcount_sum"] / duration
print(f"Total tx / duration: {implied_rate}")
```
# Transactions/Sec
```python
plots.TPS(blocks, plot_prefix=plot_prefix, timescale=timescale).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-tps.png",
    firstblock=firstblock, lastblock=lastblock
    )
```

# Block interval
```python
plots.BT(blocks, plot_prefix=plot_prefix, timescale=timescale).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-bt.png",
    logy=False, firstblock=firstblock, lastblock=lastblock
    )
```

# Block gasLimit vs gasUsed

```python
plots.GAS(blocks, plot_prefix=plot_prefix, timescale=timescale).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-gas.png",
    logy=True, firstblock=firstblock, lastblock=lastblock
    )
```

# Block size
```python
plots.BSZ(blocks, plot_prefix=plot_prefix).plot(
    matplotlib.pyplot, "{plot_prefix}-{firstblock}-{lastblock}-bsz.png",
    firstblock=firstblock, lastblock=lastblock
    )
```