*NB: I'm documenting the current format of the gosync files merely as a point in time
reference of format that is in use in the tool that is meant to serve as a practical reference and acceptance testing tool. The gosync tool is not intended as a production-worthy, well supported, tested tool.*

*The format used exists entirely in service of being able to test the implementation of the gosync library as a cohesive whole in the real world, and therefore backwards and forwards compatibility (or even efficiency) are not primary concerns.*

# Version 0.2.0
###  The header
(LE = little endian)
* The string "G0S9NC" in UTF-8
* versions*3 (eg. 0.1.2), uint16 LE 
* filesize, int64 LE
* blocksize uint32 LE

### The body
Repeating:
* WeakChecksum
* StrongChecksum

each referring to blocks, starting at 0 (file start) and going upwards.

In the current implementation of the FileChecksumGenerator used the WeakChecksum is the rolling checksum (4 bytes), and StrongChecksum is MD5 (16 bytes).
