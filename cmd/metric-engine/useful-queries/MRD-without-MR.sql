-- This query will show all Metric Report Definitions that do not have any Metric Report generated (for any reason)
.header on
select
  MRD.ID, MRD.Name, MR.Name
from MetricReportDefinition as MRD
left join MetricReport as MR on MRD.ID = MR.ReportDefinitionID
where
  MR.Name is NULL
