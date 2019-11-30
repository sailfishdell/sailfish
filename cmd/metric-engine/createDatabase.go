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
				Updates     	TEXT                  -- 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'
			)`},

		{"Create MetricMeta table", `
	  	-- These always exist
			-- They are created when the report is created
			-- multiple reports can link to the same MetricMeta (many to many relationship)
			CREATE TABLE IF NOT EXISTS MetricMeta
			(
				ID          				INTEGER UNIQUE PRIMARY KEY NOT NULL,
				Name        				TEXT NOT NULL,
				SuppressDups 				BOOLEAN NOT NULL,
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
					ReportDefID 	integer not null,
					MetricMetaID 	integer not null,

					-- indexes and constraints
					primary key (ReportDefID, MetricMetaID)
					foreign key (ReportDefID)
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
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			);`},

		{"Create MetricValue table", `
			CREATE TABLE IF NOT EXISTS MetricValue
			(
				InstanceID INTEGER NOT NULL,
				Timestamp  INTEGER NOT NULL,
				Value      VARCHAR(64) NOT NULL,

				-- indexes and constraints
				PRIMARY KEY (InstanceID, Timestamp),
				FOREIGN KEY (InstanceID)
					REFERENCES MetricInstance (ID) ON DELETE CASCADE
			)`},

		{"Create MetricReport table", `
			CREATE TABLE IF NOT EXISTS MetricReport
			(
				Name  							VARCHAR(32) PRIMARY KEY UNIQUE NOT NULL,
				ReportDefinitionID  INTEGER NOT NULL,
				Sequence 						INTEGER NOT NULL,

				-- cross reference to the start and end timestamps in the MetricValue table
				StartTimestamp   INTEGER,  -- datetime
				EndTimestamp 		 INTEGER  -- datetime
			)`},

		{"Create index for MetricReport table", `
			CREATE INDEX IF NOT EXISTS metric_report_xref_idx on MetricReport(ReportDefinitionID)`},
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
		// ============================
		_, err = database.Exec(`
				CREATE VIEW IF NOT EXISTS MetricReport_View as
						select
							mrd.Name as 'Id',
							'TODO - ' || mrd.Name || ' - Metric Report Definition' as 'Name',
							mrd.Sequence as 'ReportSequence',
							(
								select json('[' || group_concat(json_object(
										'MetricId', mvm.Name,
										'Timestamp', mv.Timestamp,
										'MetricValue', mv.MetricValue,
										'OEM', json_object('Dell', json_object(
											'ContextID', mvm.context,
											'Label', mvm.label
										))
									))  || ']') from MetricValue as mv
									inner join MetricMeta as mvm on mv.MetricMetaID == mvm.ID where mvm.ReportDefID == mrd.ID
						  ) as 'MetricValues',
							(select count(*) from MetricValue as mv
									inner join MetricMeta as mvm on mv.MetricMetaID == mvm.ID where mvm.ReportDefID == mrd.ID) as 'Metric@odata.count'
						from MetricReportDefinition  as mrd
					`)
		if err != nil {
			logger.Crit("Error executing statement for MetricReport_View create", "err", err)
			return
		}

		// =========================================
		// Create the redfish view MetricReport_JSON
		// =========================================
		_, err = database.Exec(
			`CREATE VIEW IF NOT EXISTS MetricReport_JSON as
					select json_object(
						'@odata.type','#MetricReport.v1_2_0.MetricReport',
						'@odata.context','/redfish/v1/$metadata#MetricReport.MetricReport',
						'@odata.id',  '/redfish/v1/TelemetryService/MetricReports/' || Id,
						'Id', Id,
						'Name',Name,
						'ReportSequence',ReportSequence,
						'MetricReportDefinition', json_object('@odata.id','/redfish/v1/TelemetryService/MetricReportDefinitions/' || Id),
						'Timestamp',Date('now'),
						'MetricValues', MetricValues,
						'MetricValues@odata.count', [MetricValues@odata.count]
						) as root,
							'/redfish/v1/TelemetryService/MetricReports/' || Id as '@odata.id' from MetricReport_View;
					`)
		if err != nil {
			logger.Crit("Error executing statement for MetricReport_JSON view create", "err", err)
			return
		}
	*/

	return
}
