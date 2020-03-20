set -x
IDRA_IP=$1
EN_DIS=$2

if [ "$EN_DIS" -eq "1" ]
then
	echo " Enable global Telemetry  .. "
else
	echo " Disable global Telemetry  .. "
fi

curl -s -k -u root:calvin -X PATCH https://$IDRA_IP/redfish/v1/Managers/iDRAC.Embedded.1/Attributes \
           -H 'Content-Type: application/json' -d '{ "Attributes": {"Telemetry.1.EnableTelemetry": "'$EN_DIS'"}}' | grep -i error
for rpt in NICStatistics NICSensor FCPortStatistics FCSensor FPGASensor GPUMetrics GPUStatistics NVMeSMARTData FanSensor PowerMetrics PowerStatistics ThermalSensor ThermalMetrics MemorySensor CPUSensor Sensor StorageDiskSMARTData StorageSensor CUPS CPUMemMetrics SerialLog CPURegisters PSUMetrics AggregationMetrics
do
	if [ "$EN_DIS" -eq "1" ]
	then
		echo " Enable report $rpt .. "
	else
		echo " Disable report $rpt .. "
	fi
	curl -s -k -u root:calvin -X PATCH https://$IDRA_IP/redfish/v1/Managers/iDRAC.Embedded.1/Attributes \
           -H 'Content-Type: application/json' -d '{ "Attributes": {"Telemetry'$rpt'.1.EnableTelemetry": "'$EN_DIS'"}}' | grep -i error

done
