if [ $# -ne 3 ]; then
  echo "Usage: $0 <filepath> <env_var_to_replace> <new_value>"
  exit 1
fi

FILEPATH=$1
REPLACE_ENV=$2
REPLACE_VALUE=$3

TMPFILE=$(mktemp)

# Check if equal sign is present
if grep "ENV ${REPLACE_ENV}" $FILEPATH | grep -q '='; then
  awk -F'=' 'BEGIN {OFS = FS} /ENV '"${REPLACE_ENV}"'/{$NF="'"${REPLACE_VALUE}"'"} 1' $FILEPATH > $TMPFILE && mv $TMPFILE $FILEPATH
else
  awk '/ENV '"${REPLACE_ENV}"'/{$NF="'"${REPLACE_VALUE}"'"} 1' $FILEPATH > $TMPFILE && mv $TMPFILE $FILEPATH
fi

