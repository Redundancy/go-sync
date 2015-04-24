wget https://s3-eu-west-1.amazonaws.com/gosync-test/0.1.2/gosync.exe -O test.exe
cp test.exe compare.exe
gosync b test.exe
truncate -s 5000000 test.exe  
gosync p test.exe test.gosync https://s3-eu-west-1.amazonaws.com/gosync-test/0.1.2/gosync.exe
diff -q test.exe compare.exe