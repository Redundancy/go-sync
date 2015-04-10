for f in $TRAVIS_BUILD_DIR/*test.sh
do
	[! -f "$f"] && continue

	echo "Running acceptance test $f"
done


