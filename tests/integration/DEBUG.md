# Interpreting Debug Data

## General
The debug data is going to be printed out as json that is gzipped then base64'ed. You can 
```
echo 'blob' | base64 --decode | gzip -d | jq -r
```
to get to something readable.

## Pod Logs
Pod logs are again gzip and base64'ed, but they have literal "\n" rendered in. For consumable logs, try
```
echo 'podlogblob' | base64 --decode | gzip -d | sed -e 's/SnewlineG/\n/g'
```