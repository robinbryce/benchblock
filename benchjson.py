#!/bin/bash
"""
The tuskfiles invoke this in the appropriate way to update bench.json.
"""

import os
import sys
import argparse
import pathlib
import json

def cmd_update(args):
    """write BBAKE_ vars back to bench.json

    every BBAKE_ var identitied by configvars is written back to bench.json IF
    it is set and not empty
    """

    configvars=args.configvars

    config = pathlib.Path(args.config).resolve()

    j = json.load(open(config, "r"))

    u = {}
    for k in args.configvars.split():
        v = os.environ.get(f"BBAKE_{k.upper()}")
        if not v or v == j.get(k):
            continue
        print(f"updating: {k}={v} (was {j.get(k, 'not set')})")
        u[k] = v
    j.update(u)

    with open(config, "w") as f:
        f.write(json.dumps(j, indent=2, sort_keys=True))
        f.flush()
    print(f"Wrote: {config}")


def run():
    args = sys.argv[1:]
    top = argparse.ArgumentParser(description=__doc__)
    top.set_defaults(func=lambda a, b: print("See sub commands in help"))

    subcmd = top.add_subparsers(title="Available commands")
    p = subcmd.add_parser("update", help=cmd_update.__doc__)
    p.set_defaults(func=cmd_update)
    p.add_argument("config")
    p.add_argument("configvars")

    args = top.parse_args(args)
    args.func(args)

if __name__ == "__main__":
    run()

