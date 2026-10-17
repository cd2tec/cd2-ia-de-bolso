#!/bin/sh
cd "$(dirname "$0")" || exit 1
# No pen drive com lancador este arquivo mora em core/; ai o core/ fica uma pasta acima.
[ -f core/start-unix.sh ] || cd .. || exit 1
exec sh core/start-unix.sh
