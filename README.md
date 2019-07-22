Parallel Tar ( ptar )
---
A parallel tar implementation, written in golang.

## Description
This program does not aim to create one tar file quickly, but rather many tar files, each of which is a valid tar file itself, which when all untarred recreate the original folder structure.  
#### Why this method?  
Rather than blocking on either a single serial file creation or a single directory scanner, the program can continue to scan a folder structure and output to one of the tar files via a channel, keeping the momentum up. Additionally, all the tar files are valid, so in the case of corruption some data can be recovered, and there is no special data container format. Lastly, some filtering can be applied to partition in useful ways.  
 
## Goals:
  - Fast, correct, implementation
  - Backwards compatible with GNU Tar
  - Parallel archive volumes that in-total decompress to an equivilent directory structure as classic tar
  - Index file creation
  - Manifest file creation _not done_
  - Multi-Node parallel workers _not done_
  - Parallel decompression _not done_


## Sample Performance:
```
Linux kernel source 1.5GB 47K files GPFS hot-pagepool no compression
Run     Time
gnutar  6m4s
1       6m33s
2       3m15s
4       1m30sec
8       50sec
16      23sec
32      12sec
64      9.4sec
128     9.9sec
```

```
Test dataset 10TB 500K files
Run     Time
gnutar  10hr20m
32      44m39s
32gzip  8hr22m
```

## LICENSE:
Copyright (c) 2019 Zachary Giles  
Licensed under the MIT/Expat License  
