counter=1
for f in $TRAVIS_BUILD_DIR/acceptancetests/*test.sh
do
	[ ! -f "$f" ] && continue

	echo "Running acceptance test $f"
	echo 'travis_fold:start:test_output$counter'
	sh $f
	rc=$?
	echo 'travis_fold:end:test_output$counter'
	
	if [ $rc != 0 ]; then
		echo "Test Failed"
		exit $rc
	fi

	echo "Test Passed"
	counter=`expr $counter + 1`
done


