for f in $TRAVIS_BUILD_DIR/*test.sh
do
	["$f" -eq "$TRAVIS_BUILD_DIR/*test.sh"] && continue

	echo "Running acceptance test $f"
done


