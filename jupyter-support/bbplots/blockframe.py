"""DataFrame over a blocks database
"""
import numpy

class Frame:
    def __init__(self, df):
        self._df = df


    def _add_window(self, window, field, legend, timefield="blocktime"):
        """Add a rolling average over `window' items from `field'. `legend'
        should be a format string with a place holder for {windw} and {plural}
        """

        plural="s" if window > 1 else ""
        name = legend.format(window=window, plural=plural)
        self._df[name]=self._df[field].rolling(window).sum() / self._df[timefield].rolling(window).sum()
        return name

    def df(self):
        return self._df

    def add_blocktime(self, timescale=None):
        """
        blocktime = timestamp[n] - timestamp[n-1]
        """
        if timescale is not None:
            # raft reports at 1000000s
            self._df["timestamp"] = self._df["timestamp"] / timescale

        self._df["blocktime"] = self._df["timestamp"] - self._df["timestamp"].shift()
        self._df.loc[1, "blocktime"] = numpy.nan

    def add_tps(self, window):
        """Add transactions per second indicator with data averaged over
        `window' blocks.
        """
        return self._add_window(window, "txcount", "TPS_{window}blk{plural}")

    def add_gups(self, window):
        """ gasUsed per second """
        plural="s" if window > 1 else ""
        return self._add_window(window, "gasUsed", "GUPS_{window}blk{plural}")

    def add_glps(self, window):
        plural="s" if window > 1 else ""
        return self._add_window(window, "gasLimit", "GLPS_{window}blk{plural}")

