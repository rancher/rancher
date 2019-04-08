#!/bin/bash

dest=`mktemp -d`

config-reloader \
  --interval 0 \
  --log-level=debug \
  --output-dir=${dest} \
  --templates-dir=/templates \
  --datasource=fs \
  --fs-dir=${DATASOURCE_DIR} \
  --fluentd-binary "/usr/local/bin/fluentd -p /fluentd/plugins" &> /dev/null


rc=0

for st in `find ${dest} -iname *.status`; do
  ns=$(basename ${st})
  ns=${ns%%.status}
  ns=${ns#ns-}
  echo -n "*** Error for namespace '$ns': "
  cat "${st}"
  echo
  echo
  rc=1
done

exit $rc
