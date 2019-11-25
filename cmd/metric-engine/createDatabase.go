package main

import (
	"database/sql"

	log "github.com/superchalupa/sailfish/src/log"
)

func createDatabase(logger log.Logger, dbpath string) (database *sql.DB,
	selectMetaRecordID,
	insertMeta,
	insertValue *sql.Stmt,
	err error) {

	database, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		logger.Crit("Could not open database", "err", err)
		return
	}

	// =======================
	// Create the MetricReportDefinition table
	// =======================
	statement, err := database.Prepare(
		`CREATE TABLE IF NOT EXISTS MetricReportDefinition
				(
					report_id  integer primary key not null,
					name      varcahr(64),
					reportsequence integer,
					firstTimestamp datetime,
					lastTimestamp  datetime
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

	// =======================
	// Create the MetricValuesMeta table
	// =======================
	statement, err = database.Prepare(
		`CREATE TABLE IF NOT EXISTS MetricValuesMeta
				(
					report_id integer not null,
					record_id integer unique primary key not null,
					metricid  varchar(64) not null,
					uri       varchar(255),
					property  varchar(64),
					context   varchar(64),
					label     varchar(64),
					stable_period boolean,
					reported_period datetime,
					calculated_period datetime,
					repeats_suppressed boolean,
					stopsupported boolean,
					stop boolean,
					unique (report_id, metricid, uri, property, context),
					foreign key (report_id)
						references MetricReportDefinition (record_id)
							on delete cascade
				)`)
	if err != nil {
		logger.Crit("Error Preparing statement for meta table create", "err", err)
		return
	}
	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}

	// =========================
	// Create the MetricValues table
	// =========================
	statement, err = database.Prepare(
		`CREATE TABLE IF NOT EXISTS MetricValues
				(
					record_id integer not null,
					ts datetime,
					metricvalue varchar(64),
					primary key (record_id, ts),
					foreign key (record_id)
						references MetricValuesMeta (record_id)
							on delete cascade
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

	// ======================
	// Create the MetricReport view
	// ======================
	statement, err = database.Prepare(
		`CREATE VIEW IF NOT EXISTS MetricReport_View as
					select
						'#MetricReport.v1_2_0.MetricReport' as '@odata.type',
						'/redfish/v1/$metadata#MetricReport.MetricReport' as '@odata.context',
						mrd.Name as 'Id',
						'TODO - ' || mrd.Name || ' - Metric Report Definition' as 'Name',
						mrd.reportsequence as 'ReportSequence',
						json_object('@odata.id','/redfish/v1/TelemetryService/MetricReportDefinitions/' || mrd.Name) as 'MetricReportDefinition',
						(
							select json('[' || group_concat(json_object('Timestamp', mv.ts, 'Value', mv.metricvalue ))  || ']') from MetricValues as mv
								inner join MetricValuesMeta as mvm on mv.record_id == mvm.record_id where mvm.report_id == mrd.report_id
					  ) as 'MetricValues'
					from MetricReportDefinition  as mrd
				`)
	if err != nil {
		logger.Crit("Error Preparing statement for view create", "err", err)
		return
	}
	//(json( '[' || group_concat() || ']' )) as 'MetricValues',

	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}

	// ======================
	// Create the *_JSON view
	// ======================
	statement, err = database.Prepare(
		`CREATE VIEW IF NOT EXISTS MetricReport_JSON as
				select json_object(
					'@odata.type',[@odata.type],
					'@odata.context',[@odata.context],
					'@odata.id',  '/redfish/v1/TelemetryService/MetricReports/' || Id,
					'Id', Id,
					'Name',Name,
					'ReportSequence',ReportSequence,
					'MetricReportDefinition', json_object('@odata.id','/redfish/v1/TelemetryService/MetricReportDefinitions/' || Id),
					'Timestamp',Date('now'),
					'MetricValues', MetricValues
					) as root,
						'/redfish/v1/TelemetryService/MetricReports/' || Id as '@odata.id' from MetricReport_View;
				`)
	if err != nil {
		logger.Crit("Error Preparing statement for JSON view create", "err", err)
		return
	}
	//'MetricValues',MetricValues,
	//'MetricValues@odata.count', [MetricValues@odata.count]

	_, err = statement.Exec()
	if err != nil {
		logger.Crit("Error creating table", "err", err)
		return
	}

	// ===================================
	// Select to get the correct record_id
	// ===================================
	selectMetaRecordID, err = database.Prepare(
		`Select record_id, stop from MetricValuesMeta where
			report_id=? and
			metricid=? and
			uri=?  and
			property=? and
			context=?
			`)
	if err != nil {
		logger.Crit("Error Preparing statement for find record_id in MetricValuesMeta", "err", err)
		return
	}

	// ===================================
	// Insert for new record_id
	// ===================================
	insertMeta, err = database.Prepare(
		`INSERT INTO MetricValuesMeta (
				report_id,
				metricid, uri, property, context, label, stop
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			on conflict (report_id, metricid, uri, property, context) do update SET stop=?`)
	if err != nil {
		logger.Crit("Error Preparing statement for meta table insert", "err", err)
		return
	}

	insertValue, err = database.Prepare(`INSERT INTO MetricValues (record_id, ts, metricvalue) VALUES (?, ?, ?)`)
	if err != nil {
		logger.Crit("Error Preparing statement for values table insert", "err", err)
		return
	}

	return
}
