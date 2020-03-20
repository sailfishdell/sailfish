set -x
IDRA_IP=$1

rm -rf RPTS
mkdir RPTS > /dev/null 2>&1
for rpt in NICStatistics NICSensor FCSensor FCPortStatistics FPGASensor GPUMetrics GPUStatistics NVMeSMARTData FanSensor PowerMetrics PowerStatistics ThermalSensor ThermalMetrics MemorySensor CPUSensor Sensor StorageDiskSMARTData StorageSensor CUPS CPUMemMetrics SerialLog CPURegisters PSUMetrics AggregationMetrics
do
	echo " GET report $rpt .. "
	curl -s -k -u root:calvin -X GET https://$IDRA_IP/redfish/v1/TelemetryService/MetricReports/$rpt -H 'Content-Type: application/json' > RPTS/$rpt.json
done

grep 'odata.count":  0' RPTS/*
