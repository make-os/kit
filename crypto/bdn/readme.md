This package makes use of bn256, a curve which is supposed to provide 128 bits security
but have been found to offer less (about 100-bits).

https://moderncrypto.org/mail-archive/curves/2016/000740.html 

TODO:
* Replace with BLS-based aggregated signature scheme.