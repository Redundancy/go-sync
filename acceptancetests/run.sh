for f in $TRAVIS_BUILD_DIR/acceptancetests/*test.sh
do
	[ ! -f "$f" ] && continue

	echo "Running acceptance test $f"
	sh $f

	rc=$?
	if [ $rc != 0 ] then
		exit $rc
	fi
done


