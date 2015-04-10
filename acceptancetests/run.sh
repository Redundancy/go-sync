DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

shopt -s nullglob

for f in $DIR/*test.sh
do
	echo "Running acceptance test $f"
done


