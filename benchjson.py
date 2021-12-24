#!/bin/bash
"""
The tuskfiles invoke this in the appropriate way to update bench.json.
"""

import os
import sys
import argparse
import pathlib
from pathlib import Path
import json

def cmd_update(args):
    """write BBAKE_ vars back to bench.json

    every BBAKE_ var identitied by configvars is written back to bench.json IF
    it is set and not empty
    """

    configvars=args.configvars

    config = Path(args.config).resolve()

    j = json.load(open(config, "r"))

    u = {}
    for k in args.configvars:
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


def cmd_new_config(args):
    """
    Establish a new bench.json from the cli options, the environ and the
    default configurations for the chosen consensus
    """

    missing_required = [
            opt for opt in "consensus configdir deploymode launchdir tuskdir".split()
            if getattr(args, opt) is None]
    if missing_required:
        print("These required options are missing: " + " ".join(missing_required))
        sys.exit(-1)

    tuskdir = Path(args.tuskdir).resolve()
    launchdir = Path(args.launchdir).resolve()

    configdir = launchdir.joinpath(args.configdir).resolve()
    configdir.mkdir(parents=True, exist_ok=True)
    os.chdir(configdir)

    def env(name, default=""):
      # Note that when env vars are set explicitly empty this will
      # trump the default argument here
      return os.environ.get(f"BBAKE_{name.upper()}", default)

    # if the user supplies a path, resolve it against the launchdir
    # rather than the config dir (so that the result is consistent with
    # cli tab completion)
    if args.nodesdir:
      nodesdir = launchdir.joinpath(nodesdir).resolve()
    else:
      # nodesdir defaults to configdir/<consensus>/nodes
      nodesdir = configdir.joinpath(args.consensus, "nodes").resolve()

    nodesdir.mkdir(parents=True, exist_ok=True)

    # The configuration defaults come from the appropriate benchblock
    # profile for the consensus and deployment method

    bench_json = {}

    deploymode = args.deploymode
    consensus = args.consensus

    # blockbench default profiles
    profiles = [
        (tuskdir.joinpath("configs", "default.json").resolve(), "standard"),
        (tuskdir.joinpath("configs", f"{deploymode}-default.json").resolve(), "standard"),
        (tuskdir.joinpath("configs", f"{consensus}-default.json").resolve(), "standard"),
        (tuskdir.joinpath("configs", f"{consensus}-{deploymode}-default.json").resolve(), "standard"),
        ]

    # append user profiles
    if args.profile:
        profiles.extend([
            # -p 8 will select the standard profile for an 8 node deployment of the
            # selected consensus.
            (tuskdir.joinpath("configs",
                f"{consensus}-{deploymode}-{args.profile}").resolve(), "standard"),
            (tuskdir.joinpath("configs", args.profile).resolve(), "standard"),
            # The file relative to the launchdir has the highest priority as we
            # list it last
            (launchdir.joinpath(args.profile).resolve(), "user")
            ])

    # process default profiles
    for (p, kind) in profiles:
        if not p.is_file():
            continue
        bench_json.update(json.load(open(p)))
        print(f"appyling {kind} profile: {str(p)}")

    # If its in the profile, normalise against configdir
    if "pyenv" in bench_json:
      bench_json["pyenv"] = str(configdir.joinpath(bench_json["pyenv"]))
    else:
      # otherwise, normalise from options or env against launchdir
      # pyenvdir defaults to configdir/env
      pyenvdir = env("PYENV", "env")
      pyenvdir = launchdir.joinpath(pyenvdir).resolve()
      bench_json['pyenv'] = str(pyenvdir)

    # now prioritize any explicit options to new over what is present in
    # the profile

    for k in args.configvars.split():

      if k == "consensus":
        # This is a command argument, not an option
        continue

      v = env(k, default=None)
      # v will only be None if the env var is not set. If the cli
      # option was set not empty then the env var will be set. This
      # allows env to be used to force a config value to the empty
      # string.
      if v is None:
        continue

      # if there is nothing in the profile (or we don't have one) fall
      # back to the env var (which may have been set from CLI)
      if not bench_json.get(k):
        bench_json[k] = v
        print(f"{k}: {bench_json[k]} (bbake new default)")
        continue

      # If we got a value from the cli or the env, it takes precedence
      # over anything we got from the profile
      bench_json[k] = v
      print(f"{k}: {v} (set by option, env or profile)")

    bench_json["consensus"] = consensus
    bench_json["nodesdir"] = str(nodesdir)

    # deal with any constructed defaults.
    if not bench_json.get("name"):
      bench_json["name"] = consensus + str(bench_json["maxnodes"])

    with open(configdir.joinpath("bench.json"), "w") as f:
        json.dump(bench_json, f, indent=2, sort_keys=True)

    print(json.dumps(bench_json, indent=2, sort_keys=True))
    print(f"Wrote: {configdir.joinpath('bench.json')}")


def cmd_require(args):
    """check the config file for the required values """

    configdir = Path(args.configdir).resolve()
    config = configdir.joinpath(args.config).resolve()

    j = json.load(open(config, "r"))
    missing = [var for var in args.required if var not in j]
    if missing:
        print(json.dumps(j, indent=2, sort_keys=True))
        print(str(config))
        print(f"required vars missing: {' '.join(missing)}")
        sys.exit(-1)
    sys.exit(0)


def cmd_shell_export(args):
    """
    print eval safe 'export BBAKE_{var}=value'
    """

    configdir = Path(args.configdir).resolve()
    config = configdir.joinpath(args.config).resolve()

    j = json.load(open(config, "r"))
    for k, v in j.items():
        print(f"export BBAKE_{k.upper()}={v}")


def run():
    args = sys.argv[1:]
    top = argparse.ArgumentParser(description=__doc__)
    top.set_defaults(func=lambda a, b: print("See sub commands in help"))

    subcmd = top.add_subparsers(title="Available commands")
    p = subcmd.add_parser("update", help=cmd_update.__doc__)
    p.set_defaults(func=cmd_update)
    p.add_argument("config")
    p.add_argument("configvars", nargs="+")

    p = subcmd.add_parser("require", help=cmd_require.__doc__)
    p.set_defaults(func=cmd_require)
    p.add_argument("--configdir", default=os.getcwd())
    p.add_argument("config")
    p.add_argument("required", nargs="+")

    p = subcmd.add_parser("shell-export", help=cmd_shell_export.__doc__)
    p.set_defaults(func=cmd_shell_export)
    p.add_argument("--configdir", default=os.getcwd())
    p.add_argument("config")

    p = subcmd.add_parser("new-config", help=cmd_new_config.__doc__)
    p.set_defaults(func=cmd_new_config)
    p.add_argument("config")
    p.add_argument("configvars")
    p.add_argument("--tuskdir")
    p.add_argument("--launchdir")
    p.add_argument("--configdir")
    p.add_argument("--consensus", choices=["ibft", "raft", "rrr"])
    p.add_argument("--deploymode", choices=["k8s", "compose"])
    p.add_argument("--nodesdir")
    p.add_argument("--profile")


    args = top.parse_args(args)
    args.func(args)

if __name__ == "__main__":
    run()
