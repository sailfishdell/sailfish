package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"

	log "github.com/superchalupa/sailfish/src/log"
)

type MRDMetric []struct {
	CollectionDuration  time.Duration
	CollectionFunction  string
	CollectionTimeScope string
	MetricID            string
	// future: MetricProperties []struct{}
}

func (m MRDMetric) Value() (driver.Value, error) {
	b, err := json.Marshal(m)
	return b, err
}

func (m *MRDMetric) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), m)
}

type ReportActions []string

type MetricReportDefinitionData struct {
	Name        string `db:"Name"`
	Enabled     bool   `db:"Enabled"`
	AppendLimit int    `db:"AppendLimit"`

	// 'Periodic', 'OnChange', 'OnRequest'
	MetricReportDefinitionType string

	// only for periodic reports
	MetricReportHeartbeatInterval time.Duration

	// dont put in duplicate Values for consecutive timestamps
	Suppress bool

	// 	'LogToMetricReportsCollection', 'RedfishEvent'
	ReportActions ReportActions

	// 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'
	//  we dont support 'NewReport' (for now?)
	ReportUpdates string

	ReportTimespan time.Duration
	Schedule       string    // TODO
	Metrics        MRDMetric `db:"Metrics"`
}

type MetricReportDefinition struct {
	*MetricReportDefinitionData
	*MRDFactory
	logger log.Logger
	ID     int64 `db:"ID"`
	loaded bool
}

type MetricMap map[string][]int64

func (MRD *MetricReportDefinition) EnsureMetricValueMeta(mv *MetricValueEventData) (recordID int64, err error) {
	var stop bool
	row := MRD.selectMetaRecordID.QueryRow(MRD.ID, mv.MetricID, mv.URI, mv.Property, mv.Context)
	err = row.Scan(&recordID, &stop)
	if err != nil || stop != mv.Stop {
		if err == sql.ErrNoRows {
			var res sql.Result
			res, err = MRD.insertMeta.Exec(MRD.ID, mv.MetricID, mv.URI, mv.Property, mv.Context, mv.Label, mv.Stop, mv.Stop)
			if err != nil {
				MRD.logger.Crit("ERROR inserting MetricValueMeta", "MetricValueEventData", mv, "MRD.ID", MRD.ID, "err", err)
				return
			}

			// if we had to insert new meta record, get the id for that record and return it
			recordID, err = res.LastInsertId()
			if err != nil {
				MRD.logger.Crit("ERROR getting last insert value for MetricValueMeta", "MetricValueEventData", mv, "MRD.ID", MRD.ID, "err", err)
				return
			}
		} else if stop != mv.Stop {
			// TODO: update the stop field
		} else {
			MRD.LoadFromDB()
			MRD.logger.Crit("Error scanning for metric record id", "err", err, "Report", MRD.Name, "metric id", mv.MetricID)
		}
	}
	return
}

func (MRD *MetricReportDefinition) InsertMetricValue(mv *MetricValueEventData) {
	// TODO: implement un-suppress here
	// TODO: implement suppress here
	// TODO: honor AppendLimit
	// TODO: honor AppendStopsWhenFull | AppendWrapsWhenFull | Overwrite
	// TODO: honor Type=OnChange (send an event?)
	// TODO: implement calculation functions

	recordID, err := MRD.EnsureMetricValueMeta(mv)
	if err != nil {
		MRD.logger.Crit("ERROR inserting metric meta", "err", err)
		return
	}

	// implement calculations here by using scratchpad area in metricvaluemeta
	// if calculation period isn't over, set a timer? or should we be completely event based? (probably)

	// overwrite here by "generating" the old report and then adding
	// Generate(old start, old end)

	// AppendStopsWhenFull by checking # before inserting here
	// can we do that in the insert query?
	_, err = MRD.insertValue.Exec(recordID, mv.Timestamp, mv.MetricValue)
	if err != nil {
		MRD.logger.Crit("ERROR inserting value", "err", err)
		return
	}

	// OnChange here by "generating"
	// end period == this entry

	// FINAL: remove any entries > append limit
	//   --> this satisfies AppendWrapsWhenFull

}

func (MRD *MetricReportDefinition) UpdateMetricMap(mm MetricMap) {
	// TODO: when we support more expressive filtering (ie. propery-based), add wildcard entry if needed: mm['*'] = [MRD, ...]
	MRD.LoadFromDB()

	for _, m := range MRD.Metrics {
		ary, ok := mm[m.MetricID]
		if !ok {
			ary = []int64{}
		}
		ary = append(ary, MRD.ID)
		mm[m.MetricID] = ary
	}
}

func (MRD *MetricReportDefinition) IsEnabled() bool {
	MRD.LoadFromDB()
	return MRD.Enabled
}

func (MRD *MetricReportDefinition) MatchMetric(mv *MetricValueEventData) bool {
	MRD.LoadFromDB()

	for _, m := range MRD.Metrics {
		if m.MetricID == mv.MetricID {
			return true
		}
	}

	return false
}

func (MRD *MetricReportDefinition) LoadFromDB() error {
	if MRD.loaded {
		return nil
	}

	err := MRD.database.Get(MRD, `Select Name, Enabled, AppendLimit, Metrics from MetricReportDefinition where ID=?`, MRD.ID)
	if err != nil {
		MRD.logger.Crit("failed to load metric report definition from database", "err", err)
		return err
	}

	MRD.loaded = true
	return nil
}

// Factory manages getting/putting into db

type MRDFactory struct {
	logger             log.Logger
	database           *sqlx.DB
	selectMetaRecordID *sqlx.Stmt
	insertMeta         *sqlx.Stmt
	insertValue        *sqlx.Stmt
	mm                 MetricMap
}

func NewMRDFactory(logger log.Logger, database *sqlx.DB) (ret *MRDFactory, err error) {
	ret = &MRDFactory{logger: logger, database: database, mm: MetricMap{}}
	err = nil

	// =============================================
	// Find an existing MetricMetaID for this metric
	// =============================================
	ret.selectMetaRecordID, err = database.Preparex(
		`Select ID, stop from MetricValuesMeta where
			ID=? and
			metricid=? and
			uri=?  and
			property=? and
			context=?
			`)
	if err != nil {
		logger.Crit("Error Preparing statement for find ID in MetricValuesMeta", "err", err)
		return
	}

	// ===================================
	// Insert for new MetricMetaID
	// ===================================
	ret.insertMeta, err = database.Preparex(
		`INSERT INTO MetricValuesMeta (
				ReportDefID, metricid, uri, property, context, label, stop
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			on conflict (ReportDefID, metricid, uri, property, context) do update SET stop=?`)
	if err != nil {
		logger.Crit("Error Preparing statement for meta table insert", "err", err)
		return
	}

	// ===================================
	// Insert one Metric Value record
	// ===================================
	ret.insertValue, err = database.Preparex(`INSERT INTO MetricValues (MetricMetaID, Timestamp, MetricValue) VALUES (?, ?, ?)`)
	if err != nil {
		logger.Crit("Error Preparing statement for values table insert", "err", err)
		return
	}

	return
}

func (factory *MRDFactory) Delete(mrdEvData *MetricReportDefinitionData) (err error) {
	statement, err := factory.database.Prepare(`delete from MetricReportDefinition where name==?`)
	if err != nil {
		factory.logger.Crit("Error Preparing statement for MetricReportDefinition table delete", "err", err)
		return
	}
	_, err = statement.Exec(mrdEvData.Name)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err)
		return
	}
	return
}

func (factory *MRDFactory) IterReportDefsForMetric(mv *MetricValueEventData, fn func(i *MetricReportDefinition)) {
	for _, mrd_id := range factory.mm[mv.MetricID] {
		// "skinny" MRD (not loaded from DB yet)
		MRD := &MetricReportDefinition{
			MRDFactory: factory,
			logger:     factory.logger,
			ID:         mrd_id,
		}
		fn(MRD)
	}

	for _, mrd_id := range factory.mm["*"] {
		// "skinny" MRD (not loaded from DB yet)
		MRD := &MetricReportDefinition{
			MRDFactory: factory,
			logger:     factory.logger,
			ID:         mrd_id,
		}
		// have to actually call the match function
		if MRD.MatchMetric(mv) {
			fn(MRD)
		}
	}

}

func (factory *MRDFactory) IterReportDefs(fn func(fn *MetricReportDefinition)) {
	stmt, err := factory.database.Preparex(
		`select ID, Name, Enabled, AppendLimit, Metrics from MetricReportDefinition`)
	if err != nil {
		factory.logger.Crit("Error preparing", "err", err)
		return
	}
	rows, err := stmt.Queryx()
	if err != nil {
		factory.logger.Crit("Error querying", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		MRD := &MetricReportDefinition{MRDFactory: factory, logger: factory.logger}
		rows.StructScan(MRD)
		MRD.loaded = true
		fn(MRD)
	}
}

func (factory *MRDFactory) Update(mrdEvData *MetricReportDefinitionData) (MRD *MetricReportDefinition, err error) {
	MRD = &MetricReportDefinition{
		MetricReportDefinitionData: mrdEvData,
		MRDFactory:                 factory,
		logger:                     factory.logger,
	}

	// ===================================
	// Insert for new report definition
	// ===================================
	tx, err := factory.database.Beginx()
	if err != nil {
		factory.logger.Crit("Error creating transaction to update MRD", "err", err)
		return
	}

	// if we error out at all, roll back
	defer tx.Rollback()

	statement, err := tx.PrepareNamed(
		`INSERT INTO MetricReportDefinition
					  ( Name,  Enabled, AppendLimit, Metrics)
						VALUES (:Name, :Enabled, :AppendLimit, :Metrics)
		 on conflict(name) do update set Enabled=:Enabled`)
	if err != nil {
		factory.logger.Crit("Error Preparing statement for MetricReportDefinition table insert", "err", err)
		return
	}
	res, err := statement.Exec(MRD)
	if err != nil {
		factory.logger.Crit("ERROR inserting MetricReportDefinition", "MetricReportDefinition", MRD, "err", err)
		return
	}
	if MRD.ID == 0 {
		MRD.ID, err = res.LastInsertId()
		if err != nil {
			factory.logger.Crit("ERROR getting last insert value for MetricReportDefinition", "MetricReportDefinition", MRD, "err", err)
			return
		}
	}
	tx.Commit()

	factory.mm = MetricMap{}
	factory.IterReportDefs(func(mrd *MetricReportDefinition) {
		if !mrd.IsEnabled() {
			return
		}
		mrd.UpdateMetricMap(factory.mm)
	})

	return
}
