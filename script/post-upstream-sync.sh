#!/bin/bash
#
# Script to sync with restic's upstream source
#
set -e

RESTIC_SOURCE=../restic

fix_paths() {
  gomove -d "$1" github.com/restic/restic github.com/konidev20/rapi
  gomove -d "$1" github.com/konidev20/rapi/internal/crypto github.com/konidev20/rapi/crypto
  gomove -d "$1" github.com/konidev20/rapi/internal/repository github.com/konidev20/rapi/repository 
  gomove -d "$1" github.com/konidev20/rapi/internal/restic github.com/konidev20/rapi/restic
  gomove -d "$1" github.com/konidev20/rapi/internal/backend github.com/konidev20/rapi/backend
  gomove -d "$1" github.com/konidev20/rapi/internal/pack github.com/konidev20/rapi/pack
  gomove -d "$1" github.com/konidev20/rapi/internal/walker github.com/konidev20/rapi/walker
}

# Sync rapi's public modules
for dir in walker restic crypto repository pack backend; do
  rsync -a $RESTIC_SOURCE/internal/$dir/ $dir/
  fix_paths $dir 
done


# Sync the rest of the modules
rsync -a $RESTIC_SOURCE/internal/ internal/
# These where made public
rm -rf internal/restic
rm -rf internal/pack
rm -rf internal/backend
rm -rf internal/crypto
rm -rf internal/repository
rm -rf internal/walker
fix_paths internal