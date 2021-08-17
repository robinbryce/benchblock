"""
The plots in this file are (intentionaly) replicas of the standard 4x4 plot
provided by the chain hammer project. They owe much to those efforts
https://github.com/drandreaskrueger/chainhammer
MIT License

"""
import matplotlib.pyplot
from bbench.blockframe import Frame

class TPS:
    def __init__(self, blocks, plotprefix=""):
        self._blocks = blocks
        self.plotprefix = plotprefix

    def plot(self, plt, savefile=None, firstblock=None, lastblock=None):

        firstblock, lastblock = self._blocks.checkrange(firstblock=firstblock, lastblock=lastblock)

        f = Frame(self._blocks.load_frame(firstblock=firstblock, lastblock=lastblock))

        f.add_blocktime()
        tps_cols = []
        for window in [1, 3, 5, 10]:
            tps_cols.append(f.add_tps(window))

        df = f.df()

        averages = df[tps_cols][firstblock:lastblock].mean()
        legend = [col + " (av %.1f)" % averages[col] for col in tps_cols]

        cols = ["blocknumber"] + tps_cols
        ax = df[cols][firstblock:lastblock].plot(x="blocknumber", rot=90)
        ax.set_title("transactions per second")
        ax.get_xaxis().get_major_formatter().set_useOffset(False)
        ax.legend(legend)
        # plt.tight_layout(pad=6.0, w_pad=6.0, h_pad=7.5)
        if savefile is not None:
            plt.savefig(savefile.format(plotprefix=self.plotprefix, firstblock=firstblock, lastblock=lastblock))


class BT:

    def __init__(self, blocks, plotprefix=""):
        self._blocks = blocks
        self.plotprefix = plotprefix

    def plot(self, plt, savefile=None, logy=True, firstblock=None, lastblock=None):

        firstblock, lastblock = self._blocks.checkrange(firstblock=firstblock, lastblock=lastblock)
        f = Frame(self._blocks.load_frame(firstblock=firstblock, lastblock=lastblock))

        f.add_blocktime()

        df = f.df()

        firstblock, lastblock = self._blocks.checkrange(firstblock=firstblock, lastblock=lastblock)

        kind = "bar" if (lastblock - firstblock) < 2000 else "line"

        ax=df[['blocknumber', 'blocktime']][firstblock:lastblock].plot(
            x='blocknumber', kind=kind, logy=logy
            )
    
        ax.set_title("blocktime since last block")
        ax.locator_params(nbins=1, axis='x')

        # plt.tight_layout(pad=6.0, w_pad=6.0, h_pad=7.5)
        if savefile is not None:
            plt.savefig(savefile.format(plotprefix=self.plotprefix, firstblock=firstblock, lastblock=lastblock))


class BSZ: 

    def __init__(self, blocks, plotprefix=""):
        self._blocks = blocks
        self.plotprefix = plotprefix

    def plot(self, plt, savefile=None, firstblock=None, lastblock=None):

        firstblock, lastblock = self._blocks.checkrange(firstblock=firstblock, lastblock=lastblock)
        f = Frame(self._blocks.load_frame(firstblock=firstblock, lastblock=lastblock))
        df = f.df()

        firstblock, lastblock = self._blocks.checkrange(firstblock=firstblock, lastblock=lastblock)

        kind = "bar" if (lastblock - firstblock) < 2000 else "line"

        ax=df[["blocknumber", "size"]][firstblock:lastblock].plot(x="blocknumber", rot=90, kind=kind)
        # ax.get_xaxis().get_major_formatter().set_useOffset(False)
        ax.get_yaxis().get_major_formatter().set_scientific(False)
        ax.set_title("blocksize in bytes")
        ax.locator_params(nbins=1, axis='x')

        # plt.tight_layout(pad=6.0, w_pad=6.0, h_pad=7.5)
        if savefile is not None:
            plt.savefig(savefile.format(plotprefix=self.plotprefix, firstblock=firstblock, lastblock=lastblock))


class GAS: 

    def __init__(self, blocks, plotprefix=""):
        self._blocks = blocks
        self.plotprefix = plotprefix

    def plot(self, plt, savefile=None, logy=True, firstblock=None, lastblock=None):

        f = Frame(self._blocks.load_frame(firstblock=firstblock, lastblock=lastblock))
        df = f.df()

        f.add_blocktime()

        gps_cols = []
        for window in [1, 3, 5]:
            gps_cols.append(f.add_gups(window))
        
        glps_cols = []
        for window in [1, 3, 5]:
            glps_cols.append(f.add_glps(window))

        firstblock, lastblock = self._blocks.checkrange(firstblock=firstblock, lastblock=lastblock)

        # gas
        ax=df[['blocknumber', 'GLPS_1blk', 'GUPS_1blk']][firstblock:lastblock].plot(
            x='blocknumber', rot=90, logy=logy)
        ax.get_xaxis().get_major_formatter().set_useOffset(False)
        if not logy:
            ax.get_yaxis().get_major_formatter().set_scientific(False)
        ax.set_title("gasUsed and gasLimit per second")

        # plt.tight_layout(pad=6.0, w_pad=6.0, h_pad=7.5)
        if savefile is not None:
            plt.savefig(savefile.format(plotprefix=self.plotprefix, firstblock=firstblock, lastblock=lastblock))
