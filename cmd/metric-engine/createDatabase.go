package main

import (
	"github.com/jmoiron/sqlx"

	log "github.com/superchalupa/sailfish/src/log"
)

func createDatabase(logger log.Logger, dbpath string) (database *sqlx.DB, err error) {

	database, err = sqlx.Open("sqlite3", dbpath)
	if err != nil {
		logger.Crit("Could not open database", "err", err)
		return
	}

	// TODO:  enable WAL

	// =======================================
	// Create the MetricReportDefinition table
	// =======================================
	statement, err := database.Prepare(
		`
		-- 0 - use compiled in defaults (1|2)
		-- 1 - file
		-- 2 - memory
		-- likely that 2 may be faster, but corruption is likely on crashes
		-- for now: store temp stuff in tmpfs. Need to benchmark this vs '2'
		PRAGMA TEMP_STORE = 1;

		-- TRUNCATE, DELETE, PERSIST, MEMORY, OFF
		-- Probably cant use PERSIST due to it using more space
		-- OFF == likely corruption on crashes
		PRAGMA JOURNAL_MODE = TRUNCATE;

		PRAGMA journal_mode = WAL;

		-- FULL, NORMAL, OFF
		-- OFF - corruption on OS crash, which we dont care about because it's tmpfs
		PRAGMA SYNCHRONOUS = OFF;

		-- NORMAL, EXLUSIVE
		PRAGMA LOCKING_MODE = NORMAL;

		CREATE TABLE IF NOT EXISTS MetricReportDefinition
				(
					ID    INTEGER PRIMARY KEY NOT NULL,

					-- text name of the report definition. used also for the metric report name
					Name           varcahr(64) UNIQUE,

					Enabled        BOOLEAN,
					AppendLimit    INTEGER,

					-- type of report: "Periodic", "OnChange", "OnRequest"
					Type  varcahr(64),

					Heartbeat      datetime,
					SuppressDups   BOOLEAN,

					-- the current seq of generated reports
					Sequence integer,

					-- json array of actions
					-- 'LogToMetricReportsCollection', 'RedfishEvent'
					Actions     TEXT,

					-- 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'
					Updates     TEXT,

					-- Only for 'Periodic' reports
					FirstTimestamp datetime,
					LastTimestamp  datetime,

					-- json array of metrics
					Metrics  TEXT
				)`)
	if err != nil {
		logger.Crit("Error Preparing statement for reportdefinition table create", "err", err)
		return
	}
	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}

	// =================================
	// Create the MetricValuesMeta table
	// =================================
	statement, err = database.Prepare(
		`CREATE TABLE IF NOT EXISTS MetricValuesMeta
				(
					ID          				integer unique primary key not null,
					ReportDefID  				integer not null,

					-- actually more of a metric name
					metricid     				varchar(64) not null,
					uri          				varchar(255),
					property     				varchar(64),

					-- Scratch space used by calculation functions
					scratchspace 				varchar(64),

					-- context is usually the FQDD
					context      				varchar(64),

					-- label is usually the friendly FQDD plus metric name
					label        				varchar(64),

					-- set by upstream if they avoid sending repeats and we should insert them
					-- we can only reconstruct repeats IFF there is a stable period
					repeats_suppressed 	boolean,

					-- if measurements are (assumed to be) sent on regular periods
					stable_period      	boolean,

					-- if upstream has a periodic rate, they should set this
					reported_period    	datetime,

					-- if upstream doesn't set reported_period, we'll set this based on our observations
					calculated_period  	datetime,

					-- if upstream can tell us for this stream specifically if we stop getting measurements
					stopsupported      	boolean,

					-- if this metric has stopped and we should not fill in missing data
					stop 								boolean,

					-- indexes and constraints
					unique (ReportDefID, metricid, uri, property, context),
					foreign key (ReportDefID)
						references MetricReportDefinition (ID)
							on delete cascade

				);
			CREATE INDEX idx_metricvaluesmeta_reportdefid on MetricValuesMeta(ReportDefID);`)
	if err != nil {
		logger.Crit("Error Preparing statement for meta table create", "err", err)
		return
	}
	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}
	statement.Close()

	// =============================
	// Create the MetricValues table
	// =============================
	statement, err = database.Prepare(
		`CREATE TABLE IF NOT EXISTS MetricValues
				(
					MetricMetaID INTEGER NOT NULL,
					Timestamp    DATETIME,
					MetricValue  VARCHAR(64),

					PRIMARY KEY (MetricMetaID, Timestamp),
					FOREIGN KEY (MetricMetaID)
						REFERENCES MetricValuesMeta (ID)
							ON DELETE CASCADE
				)`)
	if err != nil {
		logger.Crit("Error Preparing statement for value table create", "err", err)
		return
	}
	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}
	statement.Close()

	// ============================
	// Create the MetricReport view
	// ============================
	statement, err = database.Prepare(
		`CREATE VIEW IF NOT EXISTS MetricReport_View as
					select
						mrd.Name as 'Id',
						'TODO - ' || mrd.Name || ' - Metric Report Definition' as 'Name',
						mrd.Sequence as 'ReportSequence',
						(
							select json('[' || group_concat(json_object(
									'MetricId', mvm.metricid,
									'Timestamp', mv.Timestamp,
									'MetricValue', mv.MetricValue,
									'OEM', json_object('Dell', json_object(
										'ContextID', mvm.context,
										'Label', mvm.label
									))
								))  || ']') from MetricValues as mv
								inner join MetricValuesMeta as mvm on mv.MetricMetaID == mvm.ID where mvm.ReportDefID == mrd.ID
					  ) as 'MetricValues',
						(select count(*) from MetricValues as mv
								inner join MetricValuesMeta as mvm on mv.MetricMetaID == mvm.ID where mvm.ReportDefID == mrd.ID) as 'MetricValues@odata.count'
					from MetricReportDefinition  as mrd
				`)
	if err != nil {
		logger.Crit("Error Preparing statement for MetricReport_View create", "err", err)
		return
	}

	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}
	statement.Close()

	// =========================================
	// Create the redfish view MetricReport_JSON
	// =========================================
	statement, err = database.Prepare(
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
		logger.Crit("Error Preparing statement for JSON view create", "err", err)
		return
	}

	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}
	statement.Close()

	return
}
