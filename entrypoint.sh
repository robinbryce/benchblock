#!/bin/bash
# We need PWD to be set by the shel before invoking tusk else launchdir is
# empty
exec /usr/local/bin/tusk -qf /bbake/tusk.yml "$@"
