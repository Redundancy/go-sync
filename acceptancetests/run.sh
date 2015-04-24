for f in $TRAVIS_BUILD_DIR/acceptancetests/*test.sh
do
	[ ! -f "$f" ] && continue

	echo "Running acceptance test $f"
	echo 'travis_fold:start:$f'
	sh $f
	echo 'travis_fold:end:$f'
	rc=$?

	if [ $rc != 0 ]; then
		echo "Test Failed"
		exit $rc
	fi

	echo "Test Passed"
done


