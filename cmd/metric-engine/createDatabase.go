package main

import (
	"github.com/jmoiron/sqlx"

	log "github.com/superchalupa/sailfish/src/log"
)

func createDatabase(logger log.Logger, dbpath string) (database *sqlx.DB, err error) {

	// FOR NOW: We are going to encode the database open PRAGMA into the sqlite
	// connection string. I don't quite like that split design, but we'll do it
	// for now and go clean it up later.
	database, err = sqlx.Open("sqlite3", dbpath)
	if err != nil {
		logger.Crit("Could not open database", "err", err)
		return
	}

	// run sqlite with only one connection to avoid locking issues
	// If we run in WAL mode, you can only do one connection. Seems like a base
	// library limitation that's reflected up into the golang implementation.
	// SO: we will ensure that we have ONLY ONE GOROUTINE that does transactions
	// This isn't a terrible limitation as it is sort of what we want to do
	// anyways.
	database.SetMaxOpenConns(1)

	tables := []struct{ Comment, SQL string }{
		{"DATABASE SETTINGS", `
			PRAGMA journal_size_limit=1048576;
			PRAGMA foreign_keys = ON;
			PRAGMA journal_mode = WAL;
			PRAGMA synchronous = OFF;
			PRAGMA busy_timeout = 1000;
			`},
		{"Create MetricReportDefinition table", `
		 	CREATE TABLE IF NOT EXISTS MetricReportDefinition
			(
				ID    INTEGER PRIMARY KEY NOT NULL,
				Name        	TEXT UNIQUE NOT NULL, -- Name of the metric report defintion. This is what shows up in the collection
				Enabled     	BOOLEAN,
				AppendLimit 	INTEGER,
				Type  				TEXT,                 -- type of report: "Periodic", "OnChange", "OnRequest"
				SuppressDups	BOOLEAN,
				Actions     	TEXT,                 -- json array of options: 'LogToMetricReportsCollection', 'RedfishEvent'
				Updates     	TEXT,                 -- 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'
				Period        INTEGER
			)`},

		{"Create MetricMeta table", `
	  	-- These always exist
			-- They are created when the report is created
			-- multiple reports can link to the same MetricMeta (many to many relationship)
			CREATE TABLE IF NOT EXISTS MetricMeta
			(
				ID          				INTEGER UNIQUE PRIMARY KEY NOT NULL,
				Name        				TEXT,
				SuppressDups 				BOOLEAN NOT NULL DEFAULT true,
				PropertyPattern   	TEXT,   -- /redfish/v1/some/uri/{with}/{wildcards}#Property
				Wildcards        		TEXT,   --{"wildcard": ["array","of", "possible", "replacements"], "with": ["another", "list", "of", "replacements"]}
				CollectionFunction 	TEXT not null,   -- "sum", "avg", "min", "max"
				CollectionDuration  INTEGER,

				-- indexes and constraints
				unique (Name, SuppressDups, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration)
			)`},

		{"Create ReportDefinitionToMetricMeta table", `
			CREATE TABLE IF NOT EXISTS ReportDefinitionToMetricMeta
				(
					ReportDefinitionID 	integer not null,
					MetricMetaID 	integer not null,

					-- indexes and constraints
					primary key (ReportDefinitionID, MetricMetaID)
					foreign key (ReportDefinitionID)
						references MetricReportDefinition (ID)
							on delete cascade
					foreign key (MetricMetaID)
						references MetricMeta (ID)
							on delete cascade
				)`},

		{"Create index on ReportDefinitionToMetricMeta", `
			CREATE INDEX IF NOT EXISTS report_definition_2_metric_meta_metric_meta_id_idx ON ReportDefinitionToMetricMeta(MetricMetaID)`},

		//-- TODO later for MetricInstance. Features needed;
		//-- 		On a per-metric instance basis, need to store the "period" for that metric
		//--    When we put in a new metric, see if there were previous DUPS suppressed and expand them, IFF suppressdups==false
		//--    When we generate a report, see if there are any suppressed dups at the end of the report and expand them IFF suppressdups==false
		//--
		//--    Allow upstream to tell us when metrics stop
		//--       IFF suppressed=false
		//--       Go through and expand last metric
		{"Create MetricInstance table", `
			-- Created on demand as metrics come in
			-- Algorithm:
			-- On new MetricValueEvent:
			--   foreach select * from MetricMeta where mm.Name == event.Name
			--   	 if match_property(mm.property, event.Property)
			--        select ID from MetricInstance join metricmeta on metricinstance.MetaID == metricmeta.ID
			--  		  or insert into MetricInstance (based on MetricMeta), Get inserted ID
			-- 				then:
			--           insert into MetricValue (ID, TS, Value)
			CREATE TABLE IF NOT EXISTS MetricInstance
			(
				ID          				integer unique primary key not null,
				MetaID      			  integer not null,
				Name 								TEXT not null, -- actual metric name
				Property            TEXT not null, -- URI#Property
				Context      				TEXT not null, -- usually FQDD
				Label        				TEXT not null, -- "friendly FQDD" + "metric name" + "collectionfn"
				CollectionScratch   TEXT not null, -- Scratch space used by calculation functions
				FlushTime           INTEGER,       -- Time at which any aggregated data should be flushed
				LastTS 							INTEGER not null, -- Used to quickly suppress dups for this instance
				LastValue  					TEXT not null,    -- Used to quickly suppress dups for this instance

				-- indexes and constraints
				unique (MetaID, Name, Property, Context, Label)
				FOREIGN KEY (MetaID)
					REFERENCES MetricMeta (ID) ON DELETE CASCADE
			);`},

		{"Create MetricValueInt table", `
			CREATE TABLE IF NOT EXISTS MetricValueInt
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      INTEGER NOT NULL,

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			) WITHOUT ROWID;`},

		{"Create MetricValueReal table", `
			CREATE TABLE IF NOT EXISTS MetricValueReal
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      REAL    NOT NULL,

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			) WITHOUT ROWID;`},

		{"Create MetricValueText table", `
			CREATE TABLE IF NOT EXISTS MetricValueText
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      TEXT    NOT NULL,

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			) WITHOUT ROWID;`},

		{"Create MetricValue View", `
			CREATE View IF NOT EXISTS MetricValue as
				select InstanceID, Timestamp, Value from MetricValueText
				union all
				select InstanceID, Timestamp, Value from MetricValueInt
				union all
				select InstanceID, Timestamp, Value from MetricValueReal
			 `},

		{"Create MetricReport table", `
			CREATE TABLE IF NOT EXISTS MetricReport
			(
				Name  							TEXT PRIMARY KEY UNIQUE NOT NULL,
				ReportDefinitionID  INTEGER NOT NULL,
				Sequence 						INTEGER NOT NULL,
				ReportTimestamp     INTEGER,  -- datetime

				-- cross reference to the start and end timestamps in the MetricValue table
				StartTimestamp   INTEGER,  -- datetime
				EndTimestamp 		 INTEGER,  -- datetime

				-- indexes and constraints
				FOREIGN KEY (ReportDefinitionID)
					REFERENCES MetricReportDefinition (ID) ON DELETE CASCADE
			)`},

		{"Create index for MetricReport table", `
			CREATE INDEX IF NOT EXISTS metric_report_xref_idx on MetricReport(ReportDefinitionID);
			`},

		// this view is scalable: it will stream out without using extra memory
		// IFF(!) you do NOT "ORDER BY"
		{"Create MetricValueByReport (streamable) table.", `
				DROP VIEW IF EXISTS MetricValueByReport; CREATE VIEW MetricValueByReport as
					select
							MV.InstanceID,
							MV.Timestamp,
						  rd2mm.ReportDefinitionID as MRDID,
							json_object(
										'MetricId', MI.Name,
										'Timestamp', strftime('%Y-%m-%dT%H:%M:%f', MV.Timestamp/1000000000.0, 'unixepoch'),
										'MetricValue', MV.Value,
										'OEM', json_object(
											'Dell', json_object(
												'Context', MI.Context,
												'Label', MI.Label
											)
										)) as 'JSON'
					from MetricValue as MV
					inner join MetricInstance as MI on MV.InstanceID = MI.ID
					inner join MetricMeta as MM on MI.MetaID = MM.ID
					inner join ReportDefinitionToMetricMeta as rd2mm on MM.ID = rd2mm.MetricMetaID
					`},

		// This is the table that generates redfish output for PERIODIC metric reports
		// REQUISITE: ALL PERIODIC reports MUST have start and end timestamps!
		//
		// DOES NOT SCALE:  This uses a temp table to spool the metric values. memory usage scales with # of records output.
		//                 BECAUSE OF THE "ORDER BY" for the MetricValueByReport table!
		{"Create the Redfish Periodic {NewReport, Overwrite} Metric Report", `
				DROP VIEW IF EXISTS MetricReportPeriodic1_Redfish ; create view MetricReportPeriodic1_Redfish as
				select
					'/redfish/v1/TelemetryService/MetricReports/' || MR.Name as '@odata.id',
					('{' ||
							' "@odata.type": "#MetricReport.v1_2_0.MetricReport",' ||
							' "@odata.context": "/redfish/v1/$metadata#MetricReport.MetricReport",' ||
							' "@odata.id": "/redfish/v1/TelemetryService/MetricReports/' || MR.Name || '",' ||
							' "Id": "' || MR.Name || '",' ||
							' "Name": "' || MR.Name || ' Metric Report",' ||
							' "ReportSequence": ' || Sequence || ',' ||
							' "Timestamp": ' || strftime('"%Y-%m-%dT%H:%M:%f"', MR.ReportTimestamp/1000000000.0, 'unixepoch') || ', ' ||
							' "MetricReportDefinition": {"@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions/' || MRD.Name || '"}, ' ||
								(
									SELECT
										' "MetricValues":  [' ||  group_concat(a.JSON) || '], ' ||
										' "MetricValues@odata.count": ' ||  count(*)
									FROM
									(
										select JSON
										from MetricValueByReport as MVR
										where MVR.MRDID=MR.ReportDefinitionID
											and ( MVR.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
											order by MVR.InstanceID ASC, MVR.Timestamp ASC
											limit 1000
									) AS a )
						|| '}'
					) as root
				from MetricReport as MR
				inner join MetricReportDefinition as MRD on MR.ReportDefinitionID = MRD.ID
				where MRD.Type = 'Periodic' and ( MRD.Updates = "NewReport" or MRD.Updates = "Overwrite" or MRD.Updates = "AppendStopsWhenFull")
				`},

		// DOES NOT SCALE:  This uses a temp table to spool the metric values. memory usage scales with # of records output.
		{"Create the Redfish Periodic AppendWrapsWhenFull Metric Report", `
				DROP VIEW IF EXISTS MetricReportPeriodic2_Redfish ; create view MetricReportPeriodic2_Redfish as
				select
					'/redfish/v1/TelemetryService/MetricReports/' || MR.Name as '@odata.id',
					('{' ||
							' "@odata.type": "#MetricReport.v1_2_0.MetricReport",' ||
							' "@odata.context": "/redfish/v1/$metadata#MetricReport.MetricReport",' ||
							' "@odata.id": "/redfish/v1/TelemetryService/MetricReports/' || MR.Name || '",' ||
							' "Id": "' || MR.Name || '",' ||
							' "Name": "' || MR.Name || ' Metric Report",' ||
							' "ReportSequence": ' || Sequence || ',' ||
							' "Timestamp": ' || strftime('"%Y-%m-%dT%H:%M:%f"', MR.ReportTimestamp/1000000000.0, 'unixepoch') || ', ' ||
							' "MetricReportDefinition": {"@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions/' || MRD.Name || '"}, ' ||
								(
									SELECT
										' "MetricValues":  [' ||  group_concat(a.JSON) || '], ' ||
										' "MetricValues@odata.count": ' ||  count(*)
									FROM
									(
										select JSON
										from MetricValueByReport as MVR
										where MVR.MRDID=MR.ReportDefinitionID
											and ( MVR.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
											order by MVR.InstanceID ASC, MVR.Timestamp DESC
											limit 1000
									) AS a )
						|| '}'
					) as root
				from MetricReport as MR
				inner join MetricReportDefinition as MRD on MR.ReportDefinitionID = MRD.ID
				where MRD.Type = 'Periodic' and ( MRD.Updates = "AppendWrapsWhenFull" )
				`},

		// This is the table that generates redfish output for OnRequest metric reports
		// REQUISITE:
		//
		// DOES NOT SCALE:  This uses a temp table to spool the metric values. memory usage scales with # of records output.
		{"Create the Redfish OnRequest Metric Report", `
				DROP VIEW IF EXISTS MetricReportOnRequest_Redfish ; create view MetricReportOnRequest_Redfish as
				select
					'/redfish/v1/TelemetryService/MetricReports/' || MR.Name as '@odata.id',
					('{' ||
							' "@odata.type": "#MetricReport.v1_2_0.MetricReport",' ||
							' "@odata.context": "/redfish/v1/$metadata#MetricReport.MetricReport",' ||
							' "@odata.id": "/redfish/v1/TelemetryService/MetricReports/' || MR.Name || '",' ||
							' "Id": "' || MR.Name || '",' ||
							' "Name": "' || MR.Name || ' Metric Report",' ||
							' "ReportSequence": ' || Sequence || ',' ||
							' "Timestamp": ' || strftime('"%Y-%m-%dT%H:%M:%f"', MR.ReportTimestamp/1000000000.0, 'unixepoch') || ', ' ||
							' "MetricReportDefinition": {"@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions/' || MRD.Name || '"}, ' ||
								(
									SELECT
										' "MetricValues":  [' ||  group_concat(a.JSON) || '], ' ||
										' "MetricValues@odata.count": ' ||  count(*)
									FROM
									(
										select JSON
										from MetricValueByReport as MVR
										where MVR.MRDID=MR.ReportDefinitionID
											and ( MVR.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
											order by MVR.InstanceID ASC, MVR.Timestamp DESC
									) AS a)
						|| '}'
					) as root
				from MetricReport as MR
				inner join MetricReportDefinition as MRD on MR.ReportDefinitionID = MRD.ID
				where ( MRD.Type = 'OnRequest' or MRD.Type = 'OnChange' )
				`},

		// This is the table that creates a uniform table name to gather *any* metric report, regardless of type
		{"Create the Redfish Metric Report", `
				DROP VIEW IF EXISTS MetricReport_Redfish ; create view MetricReport_Redfish as
				select * from MetricReportOnRequest_Redfish
					UNION ALL
				select * from MetricReportPeriodic1_Redfish   -- periodic newreport/overwrite
					UNION ALL
				select * from MetricReportPeriodic2_Redfish   -- periodic appenstopswhenfull
				`},

		/*
			{"Create the Redfish Metric Report Definition view", `
					DROP VIEW IF EXISTS MetricReportDefinition_Redfish ; create view MetricReportDefinition_Redfish as
					select
						'/redfish/v1/TelemetryService/MetricReportDefinitions/' || MR.Name as '@odata.id',
						('{' ||
								' "@odata.type": "#MetricReport.v1_2_0.MetricReport",' ||
								' "@odata.context": "/redfish/v1/$metadata#MetricReport.MetricReport",' ||
								' "@odata.id": "/redfish/v1/TelemetryService/MetricReports/' || MR.Name || '",' ||
								' "Id": "' || MR.Name || '",' ||
								' "Name": "' || MR.Name || ' Metric Report",' ||
								' "ReportSequence": ' || Sequence || ',' ||
								' "Timestamp": ' || strftime('"%Y-%m-%dT%H:%M:%f"', MR.ReportTimestamp/1000000000.0, 'unixepoch') || ', ' ||
								' "MetricReportDefinition": {"@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions/' || MRD.Name || '"}, ' ||
								' "MetricValues": ' || (
										select json_group_array(JSON)
										from MetricValueByReport_JSON as MVRJ
										where MVRJ.MRDID=MR.ReportDefinitionID
							  			and ( MVRJ.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
											and ( MVRJ.Timestamp <= MR.EndTimestamp OR MR.EndTimestamp is NULL )
									) || ',' ||
								' "MetricValues@odata.count": ' || (
										select count(*)
										from MetricValueByReport as MVR
										where MVR.MRDID=MR.ReportDefinitionID
							  			and ( MVR.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
											and ( MVR.Timestamp <= MR.EndTimestamp OR MR.EndTimestamp is NULL )
							) ||
							'}'
						) as root
					from MetricReport as MR
					inner join MetricReportDefinition as MRD on MR.ReportDefinitionID = MRD.ID
					`}, // TODO: index on the odata.id field above
		*/

		{"Create the Redfish Metric Report VIEW for backwards compat with older telemetry service", `
				DROP VIEW IF EXISTS AggregationMetricsMRView_json ; create view AggregationMetricsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS CPUMemMetricsMRView_json ; create view CPUMemMetricsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS CPURegistersMRView_json ; create view CPURegistersMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS CPUSensorMRView_json ; create view CPUSensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS CUPSMRView_json ; create view CUPSMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS FCSensorMRView_json ; create view FCSensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS FPGASensorMRView_json ; create view FPGASensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS FanSensorMRView_json ; create view FanSensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS GPUMetricsMRView_json ; create view GPUMetricsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS GPUStatisticsMRView_json ; create view GPUStatisticsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS MemorySensorMRView_json ; create view MemorySensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS NICSensorMRView_json ; create view NICSensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS NICStatisticsMRView_json ; create view NICStatisticsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS NVMeSMARTDataMRView_json ; create view NVMeSMARTDataMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS PSUMetricsMRView_json ; create view PSUMetricsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS PowerMetricsMRView_json ; create view PowerMetricsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS PowerStatisticsMRView_json ; create view PowerStatisticsMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS SensorMRView_json ; create view SensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS StorageDiskSMARTDataMRView_json ; create view StorageDiskSMARTDataMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS StorageSensorMRView_json ; create view StorageSensorMRView_json as select * from MetricReport_Redfish;
				DROP VIEW IF EXISTS ThermalSensorMRView_json ; create view ThermalSensorMRView_json as select * from  MetricReport_Redfish;
				DROP VIEW IF EXISTS ThermalMetricsMRView_json ; create view ThermalMetricsMRView_json as select * from  MetricReport_Redfish;

				-- DROP VIEW IF EXISTS TelemetryServiceView_json ; create view TelemetryServiceView_json as select * from MetricReport_Redfish;
				-- DROP VIEW IF EXISTS TelemetryLogServiceLCLogview_json ; create view TelemetryLogServiceLCLogview_json as select * from MetricReport_Redfish;
				-- DROP VIEW IF EXISTS MetricDefinitionCollectionView_json ; create view MetricDefinitionCollectionView_json as select * from MetricReport_Redfish;
				-- DROP VIEW IF EXISTS MetricDefinitionView_json ; create view MetricDefinitionView_json as select * from MetricReport_Redfish;
				-- DROP VIEW IF EXISTS MetricReportCollectionView_json ; create view MetricReportCollectionView_json as select * from MetricReport_Redfish;
				-- DROP VIEW IF EXISTS MetricReportDefinitionCollectionView_json ; create view MetricReportDefinitionCollectionView_json as select * from MetricReport_Redfish;
				`},
	}

	for _, sqlstmt := range tables {
		_, err = database.Exec(sqlstmt.SQL)
		if err != nil {
			logger.Crit("Error executing setup SQL", "comment", sqlstmt.Comment, "err", err, "sql", sqlstmt.SQL)
			return
		}

	}

	return
}
