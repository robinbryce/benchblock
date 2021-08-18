"""This schema is identical (for now) to that used by the chainhammer project.

https://github.com/drandreaskrueger/chainhammer
"""

import sqlite3
import pandas


class DB:

    def __init__(self, dbfile):
        self._dbfile = dbfile


    def connect(self):
        return sqlite3.connect(self._dbfile)

    def query(self, stmt, c=None):
        close = False
        if c  is None:
            c = sqlite3.connect(self._dbfile)
            close = True
        try:
            cur = c.cursor()
            cur.execute(stmt)
            return cur
        finally:
            if close:
                c.close()


class Blocks:

    TABLE_NAME = "blocks"

    def __init__(self, dbfile):
        self._db = DB(dbfile)
        with self._db.connect() as c:
            cur = c.cursor()
            r = cur.execute(f"SELECT MIN(blocknumber), MAX(blocknumber) FROM {self.TABLE_NAME}")
            self._minblock, self._maxblock = r.fetchall()[0]

    def common_stats(self, firstblock=None, lastblock=None):

        where_clause = self.where_clause_blockrange(
            start=firstblock, end=lastblock)

        with self._db.connect() as c:

            r = dict(
                txcount_sum = self._db.query(f"SELECT SUM(txcount) FROM blocks WHERE {where_clause}", c=c).fetchall()[0][0],
                size_max = self._db.query(f"SELECT MAX(size) FROM blocks WHERE {where_clause}", c=c).fetchall()[0][0],
                txcount_max = self._db.query(f"SELECT MAX(txcount) FROM blocks WHERE {where_clause}", c=c).fetchall()[0][0],
                txcount_av = self._db.query(f"SELECT AVG(txcount) FROM blocks WHERE {where_clause}", c=c).fetchall()[0][0],
                txcount_av_nonempty = 0,
                blocks_nonempty_count = self._db.query(
                    f"SELECT COUNT(blocknumber) FROM blocks WHERE txcount != 0 AND {where_clause}", c=c).fetchall()[0][0]
            )
            if r["blocks_nonempty_count"] > 0:
                r["txcount_av_nonempty"] = r["txcount_sum"] / r["blocks_nonempty_count"]

            return r

    def load_frame(self, firstblock=None, lastblock=None):
        """Load a block range into a pandas data frame. Load all blocks by
        default."""

        df = None
        with self._db.connect() as c:

            where_clause = self.where_clause_blockrange(
                start=firstblock, end=lastblock)

            df = pandas.read_sql(
                f"SELECT * FROM {self.TABLE_NAME} WHERE {where_clause}"
                f" ORDER BY blocknumber", c)

        return df

    def checkrange(self, firstblock=None, lastblock=None):

        if firstblock is None:
            firstblock = self._minblock
        if lastblock is None or lastblock <= 0:
            lastblock = self._maxblock

        if firstblock < self._minblock:
            raise IndexError(f"firstblock: {firstblock} < minblock {self._minblock}")

        if lastblock > self._maxblock:
            raise IndexError(f"lastblock: {lastblock} > maxblock {self._maxblock}")

        return firstblock, lastblock

    def where_clause_blockrange(self, start=None, end=None):

        start, end = self.checkrange(firstblock=start, lastblock=end)

        return f"{start}<=blocknumber and blocknumber<={end}"
