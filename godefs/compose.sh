#!/bin/sh

echo -n '{"name": "HelloEvent", "encoding": "binary", "event_array": [ {"data": "'  > hello.json
./hello | base64  -w 0 >> hello.json
echo '"}]}' >> hello.json
