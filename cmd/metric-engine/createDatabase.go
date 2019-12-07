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
				Property            TEXT not null, -- URI#Property
				Context      				TEXT not null, -- usually FQDD
				Label        				TEXT not null, -- "friendly FQDD" + "metric name" + "collectionfn"
				CollectionScratch   TEXT not null, -- Scratch space used by calculation functions
				LastTS 							INTEGER not null, -- Used to quickly suppress dups for this instance
				LastValue  					TEXT not null,    -- Used to quickly suppress dups for this instance

				-- indexes and constraints
				unique (MetaID, Property, Context, Label)
				FOREIGN KEY (MetaID)
					REFERENCES MetricMeta (ID) ON DELETE CASCADE
			);`},

		{"Create MetricValue table", `
			CREATE TABLE IF NOT EXISTS MetricValue
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      TEXT    NOT NULL, -- sized to store 64-bit float as string

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			) WITHOUT ROWID`},

		{"Create MetricValueInt table", `
			CREATE TABLE IF NOT EXISTS MetricValueInt
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      INTEGER NOT NULL, -- sized to store 64-bit float as string

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			) WITHOUT ROWID`},

		{"Create MetricValueReal table", `
			CREATE TABLE IF NOT EXISTS MetricValueReal
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      REAL    NOT NULL, -- sized to store 64-bit float as string

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			) WITHOUT ROWID`},

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
			CREATE INDEX IF NOT EXISTS metric_report_xref_idx on MetricReport(ReportDefinitionID)`},

		{"Create MetricReport_VALUES (streamable) table.", `
				CREATE VIEW IF NOT EXISTS MetricReport_VALUES as
						select
							MRD.Name as 'MRDName',
						  MR.Name as 'ReportName',
							MR.Sequence as 'Sequence',
							MM.Name as 'MetricID',
							strftime('%Y-%m-%dT%H:%M:%S.%f', MV.Timestamp/1000000000.0, 'unixepoch' ) as 'Timestamp',
							MV.Value as 'MetricValue',
							MI.Context as 'Context',
							MI.Label as 'Label',

							json_object(
										'MetricId', MM.Name,
										'Timestamp', strftime('%Y-%m-%dT%H:%M:%f', MV.Timestamp/1000000000.0, 'unixepoch'),
										'MetricValue', MV.Value,
										'OEM', json_object(
											'Dell', json_object(
												'Context', MI.Context,
												'Label', MI.Label
											)
										)) || ',' as 'JSON'

						from MetricValue as MV
						inner join MetricInstance as MI on MV.InstanceID = MI.ID
						inner join MetricMeta as MM on MI.MetaID = MM.ID
						inner join ReportDefinitionToMetricMeta rd2mm on MM.ID = rd2mm.MetricMetaID
						inner join MetricReportDefinition as MRD on rd2mm.ReportDefinitionID = MRD.ID
						inner join MetricReport as MR on MRD.ID = MR.ReportDefinitionID
						where rd2mm.ReportDefinitionID = MRD.ID
							and ( MV.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
							and ( MV.Timestamp <= MR.EndTimestamp OR MR.EndTimestamp is NULL );

						-- NOTES:
						-- Can't do order by! Catastrophic results as sqlite reads everything into ram and then sorts.
						-- THIS BLOWS UP: order by MV.Timestamp, MM.Name, MI.Label
					`},
	}

	for _, sqlstmt := range tables {
		_, err = database.Exec(sqlstmt.SQL)
		if err != nil {
			logger.Crit("Error executing setup SQL", "comment", sqlstmt.Comment, "err", err, "sql", sqlstmt.SQL)
			return
		}

	}

	/*

		// ============================
		// Create the MetricReport view
		//
		// ---> this is a bad idea. it doesn't scale well when report size grows.
		// This spools the "metricvalues" to a temporary table in RAM and completely
		// blows up memory usage, getting killed by OOM. This happens when the report
		// gets large, but the memory usage scales per the report size.
		//
		// ============================
		_, err = database.Exec(`
					CREATE VIEW IF NOT EXISTS MetricReport_View as
							select
								MR.Name as 'Id',
								MR.Sequence as 'Sequence',
								MRD.Name as 'MRDName',
								MR.Name || ' Metric Report' as 'Name',
								strftime('%Y-%m-%dT%H:%M:%f', MR.ReportTimestamp) as 'Timestamp',
								(
									select
									  json('[' || group_concat(json_object(
											'MetricId', MM.Name,
											'Timestamp', strftime('%Y-%m-%dT%H:%M:%f', MV.Timestamp),
											'MetricValue', MV.Value,
											'OEM', json_object(
												'Dell', json_object(
													'Context', MI.Context,
													'Label', MI.Label
												)
											))) || ']' )
									from MetricValue as MV
									inner join MetricInstance as MI on MV.InstanceID = MI.ID
									inner join MetricMeta as MM on MI.MetaID = MM.ID
									inner join ReportDefinitionToMetricMeta rd2mm on MM.ID = rd2mm.MetricMetaID
									where rd2mm.ReportDefinitionID = MRD.ID
										and ( MV.Timestamp >= MR.StartTimestamp OR MR.StartTimestamp is NULL )
										and ( MV.Timestamp <= MR.EndTimestamp OR MR.EndTimestamp is NULL )
								)	as 'MetricValues',
								(
									select count(*)
									from MetricValue as MV
									inner join MetricInstance as MI on MV.InstanceID = MI.ID
									inner join MetricMeta as MM on MI.MetaID = MM.ID
									inner join ReportDefinitionToMetricMeta rd2mm on MM.ID = rd2mm.MetricMetaID
									where rd2mm.ReportDefinitionID = MRD.ID
								) as 'MetricValues@odata.count'
							from MetricReport as MR
							INNER JOIN MetricReportDefinition as MRD ON MR.ReportDefinitionID = MRD.ID
						`)
		if err != nil {
			logger.Crit("Error executing statement for MetricReport_View create", "err", err)
			return
		}

		// =========================================
		// Create the redfish view MetricReport_JSON
		//
		// --> This is even worse than above due to json
		//
		// =========================================
		_, err = database.Exec(
			`CREATE VIEW IF NOT EXISTS MetricReport_HEADER as
						select
						'{ ' || '"@odata.type": "#MetricReport.v1_2_0.MetricReport", ' ||
							'"@odata.context: "/redfish/v1/$metadata#MetricReport.MetricReport", ' ||
							'"@odata.id": "/redfish/v1/TelemetryService/MetricReports/' || Id || '", ' ||
							'"Id": "' || Id || '", ' ||
							'"Name", "' || Name || '", ' ||
							'"ReportSequence": "' || Sequence || '", ' ||
							'"MetricReportDefinition": {"@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions/' || MRDName || '"}, ' ||

							'"Timestamp": "' || Sequence || '", ' ||
							'Timestamp',Date('now'),

							) as JSON,
								'/redfish/v1/TelemetryService/MetricReports/' || Id as '@odata.id'

							from MetricReport_View;
						`)
		if err != nil {
			logger.Crit("Error executing statement for MetricReport_JSON view create", "err", err)
			return
		}



		Metrics: [
		{"MetricID": "ThermalReading", "CollectionFunction": "Average", "CollecitonDuration": "20s"},
		{"MetricID": "ThermalReading", "CollectionFunction": "Minimum", "CollecitonDuration": "20s"},
		{"MetricID": "ThermalReading", "CollectionFunction": "Maximum", "CollecitonDuration": "20s"},
		{"MetricID": "CPUCups", "CollectionFunction": "Average", "CollecitonDuration": "40s"},
		{"MetricID": "RPMReading", "CollectionFunction": "Average", "CollecitonDuration": "60s"},
		]


		MetricValue -
			TS - 1   value  - 1
			TS - 2   value  - 1.5
			TS - 2   value  - 1.5
			TS - 4   value  - 1.5
			TS - 5   value  - 1.5
			TS - 8   value  - 1.5

		Report XYZ1 : BeginTimestamp=2 EndTimestamp=5
		Report XYZ2 : BeginTimestamp=1 EndTimestamp=5
		Report XYZ1 : BeginTimestamp=2 EndTimestamp=8

		"Generate" a report (pseudocode) - Periodic report , time period = 60s
			Update MetricReport where ReportName == 'foo'  set BeginTimestamp = EndTimestamp, EndTimestamp = datetime('now');

			" ... select from MetricValue .... where Timestamp >= report.BeginTimestamp and Timestamp < Report.EndTimestamp;


		_, err = database.Exec(
			`CREATE VIEW IF NOT EXISTS MetricReport_Redfish as
						select json_object(
							'@odata.type','#MetricReport.v1_2_0.MetricReport',
							'@odata.context','/redfish/v1/$metadata#MetricReport.MetricReport',
							'@odata.id',  '/redfish/v1/TelemetryService/MetricReports/' || Id,
							'Id', Id,
							'Name',Name,
							'ReportSequence',Sequence,
							'MetricReportDefinition', json_object('@odata.id','/redfish/v1/TelemetryService/MetricReportDefinitions/' || MRDName),
							'Timestamp',Date('now'),
							'MetricValues', MetricValues,
							'MetricValues@odata.count', [MetricValues@odata.count]
							) as JSON,
								'/redfish/v1/TelemetryService/MetricReports/' || Id as '@odata.id' from MetricReport_View;
						`)
		if err != nil {
			logger.Crit("Error executing statement for MetricReport_JSON view create", "err", err)
			return
		}
	*/

	return
}
