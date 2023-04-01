#!/bin/bash
#
# Script to sync with restic's upstream source
#
set -e

RESTIC_SOURCE=../restic

fix_paths() {
  gomove -d "$1" github.com/restic/restic github.com/konidev20/restic-api
  gomove -d "$1" github.com/konidev20/restic-api/internal/crypto github.com/konidev20/restic-api/crypto
  gomove -d "$1" github.com/konidev20/restic-api/internal/repository github.com/konidev20/restic-api/repository 
  gomove -d "$1" github.com/konidev20/restic-api/internal/restic github.com/konidev20/restic-api/restic
  gomove -d "$1" github.com/konidev20/restic-api/internal/backend github.com/konidev20/restic-api/backend
  gomove -d "$1" github.com/konidev20/restic-api/internal/pack github.com/konidev20/restic-api/pack
  gomove -d "$1" github.com/konidev20/restic-api/internal/walker github.com/konidev20/restic-api/walker
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