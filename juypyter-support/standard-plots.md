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
import bbench.db
reload(bbench.db)
blocks = bbench.db.Blocks(dbfile)
for k, v in blocks.common_stats(firstblock=firstblock, lastblock=lastblock).items():
    print(f"{k}={v}")
```

```python
from importlib import reload
import bbench.db
import bbench.plots
import matplotlib.pyplot
reload(bbench.db)
reload(bbench.blockframe)
reload(bbench.plots)

matplotlib.rcParams["agg.path.chunksize"] = 10000

bbench.plots.GAS(blocks).plot(
    matplotlib.pyplot, "{plotprefix}-{firstblock}-{lastblock}-gas.png",
    logy=False, firstblock=firstblock, lastblock=lastblock
    )

bbench.plots.BSZ(blocks).plot(
    matplotlib.pyplot, "{plotprefix}-{firstblock}-{lastblock}-bsz.png",
    firstblock=firstblock, lastblock=lastblock
    )


bbench.plots.BT(blocks).plot(
    matplotlib.pyplot, "{plotprefix}-{firstblock}-{lastblock}-bt.png",
    logy=False, firstblock=firstblock, lastblock=lastblock
    )

bbench.plots.TPS(blocks).plot(
    matplotlib.pyplot, "{plotprefix}-{firstblock}-{lastblock}-tps.png",
    firstblock=firstblock, lastblock=lastblock
    )

#gps_cols = []
#for window in [1, 3, 5]:
#    gps_cols.append(bf.add_gups(window))
#
#glps_cols = []
#for window in [1, 3, 5]:
#    glps_cols.append(bf.add_glps(window))
```
