#!/bin/bash
CURDIR=$(cd $(dirname $0); pwd)
BinaryName=deyu.core_api
echo "$CURDIR/bin/${BinaryName}"
exec $CURDIR/bin/${BinaryName}