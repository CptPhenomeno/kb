#!/bin/sh
   # Print the CHANGELOG.md section for a version tag (e.g. v1.0.2).
   set -eu

   tag="${1:?usage: release-notes.sh <tag>}"
   ver="${tag#v}"   # strip the leading "v"

   awk -v v="$ver" '/^## \[/ { in_section = ($0 ~ "^## \\[" v "\\]") }in_section' CHANGELOG.md
