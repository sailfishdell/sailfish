#!/bin/sh
set -e
#set -x


if [ "$#" -ne 2 ]; then
    echo "$0 <count> <target file>"
    exit 0
fi

> $2

echo "running $1 times"

for (( ID=1; ID<=$1; ID++ ))
do
    echo '{"name": "LogEvent","event_seq": '$ID',"event_array": [{"Id": '$ID',"MessageArgs": ["3","C2"],"EventId": "2330","FQDD": "System.Modular.3#IOM.Slot.C2","Description": "A fabric type mismatch is detected between the server in slot and I/O module as identified in the message.","Created": "1554351926","Name": "Log Entry 3401","EntryType": "Oem","MessageID": "HWC2018","Action": "Check the chassis fabric type by using the Manage Module graphic user interface (GUI) and compare against the type of fabric used in the I/O or mezzanine card. To check chassis type on the Manage Module home page, select the server, and then view the I/O module properties. For more information about the compatible fabric types, see the Manage Module User&apos;s Guide available on the support site.","Message": "A fabric mismatch is detected between the server in slot 3 and IO Module in slot C2.","Severity": "Warning","Category": "System Health","LogAlert": "log"}]}' >> $2
done
