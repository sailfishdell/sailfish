package udb

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

// probably should be a configuration item
const maximport = 30

type dataSource struct {
	logger        log.Logger
	query         *sqlx.NamedStmt
	HWM           int64 `db:"HWM"`
	lastImport    time.Time
	nextImport    time.Time
	waitInterval  time.Duration
	scanInterval  time.Duration
	sourceName    string
	udbImportName string
	dbChange      map[string]map[string]struct{}
	importFn      func() error
	sendFn        func(*[]eh.EventData)
	maxImport     int
	syncEvent     bool
}

func (ds *dataSource) ProcessDBChange(database, table string) (err error) {
	tbls, ok := ds.dbChange[database]
	if !ok {
		return nil
	}
	_, ok = tbls[table]
	if !ok {
		return nil
	}
	// force import either immediately or at the next wait interval depending on how long it's been
	ds.nextImport = ds.lastImport.Add(ds.waitInterval)
	return ds.PeriodicImport(false)
}

func (ds *dataSource) PeriodicImport(periodic bool) (err error) {
	now := time.Now()

	if ds.nextImport.IsZero() || now.Before(ds.nextImport) {
		if periodic && ds.scanInterval == 0 {
			return
		}
		if periodic && now.Before(ds.lastImport.Add(ds.scanInterval)) {
			return
		}
		if now.Before(ds.lastImport.Add(ds.waitInterval)) {
			return
		}
	}

	// set lastImport *after* the import is done, so importFn can use that value accurately
	defer func() {
		ds.lastImport = now
		ds.nextImport = time.Time{}
		if ds.scanInterval > 0 {
			ds.nextImport = now.Add(ds.scanInterval)
		}
	}()
	return ds.importFn()
}

// dataSourceConfig mirrors exactly what we expect in the config file yaml.
// we'll unmarshal to this, then validate/parse into a dataSource which has
// more semantic meaning.
type dataSourceConfig struct {
	Type         string
	Query        string
	WaitInterval int64
	ScanInterval int64
	DBChange     map[string]map[string]struct{}
	SourceName   string
	MaxImport    int
	SyncEvent    bool
}

//nolint:unparam // has to conform to the constructor API
func newDisabledSource(log.Logger, *sqlx.DB, busComponents, *viper.Viper, string) (DataImporter, error) {
	return &dataSource{importFn: func() error { return disabled }}, nil
}

func commonMakeNewSource(logger log.Logger, database *sqlx.DB, d busComponents, cfg *viper.Viper, sect string) (*dataSource, error) {
	settings, err := unmarshalSourceCfg(cfg)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse config section(UDB-Metric-Import.%s): %s", sect, err)
	}

	meta := &dataSource{
		logger:        logger,
		scanInterval:  time.Duration(settings.ScanInterval) * time.Second,
		waitInterval:  time.Duration(settings.WaitInterval) * time.Second,
		dbChange:      settings.DBChange,
		sourceName:    settings.SourceName,
		sendFn:        makeSendFunction(logger, d.GetBus(), settings.MaxImport, settings.SyncEvent),
		maxImport:     settings.MaxImport,
		syncEvent:     settings.SyncEvent,
		udbImportName: sect,
	}

	// set sourceName to default to the section name if not set in config
	if meta.sourceName == "" {
		meta.sourceName = sect
	}

	meta.query, err = database.PrepareNamed(settings.Query)
	if err != nil {
		return nil, xerrors.Errorf("prepare() failed for query Section(%s), key(%s), sql query(%s): %w", "UDB-Metric-Import", sect, settings.Query, err)
	}

	return meta, nil
}

func makeSendFunction(logger log.Logger, bus eh.EventBus, maximport int, syncEvent bool) func(*[]eh.EventData) {
	return func(events *[]eh.EventData) {
		if len(*events) == 0 {
			return
		}
		evt := event.NewSyncEvent(metric.MetricValueEvent, *events, time.Now())
		evt.Add(1)
		err := bus.PublishEvent(context.Background(), evt)
		if err != nil {
			logger.Crit("Error publishing event to internal event bus. Should never happen!", "err", err)
		}
		if syncEvent {
			evt.Wait()
		}
		*events = make([]eh.EventData, 0, maximport/2)
	}
}
func unmarshalSourceCfg(cfg *viper.Viper) (*dataSourceConfig, error) {
	cfg.SetDefault("Type", "MetricColumns")
	cfg.SetDefault("WaitInterval", "5")
	cfg.SetDefault("ScanInterval", "0")
	cfg.SetDefault("MaxImport", maximport)
	settings := dataSourceConfig{}
	err := cfg.Unmarshal(&settings)
	if err != nil {
		return nil, err
	}

	// have to specifically unmarshal dbchange for some weird reason
	err = cfg.UnmarshalKey("dbchange", &settings.DBChange)
	if err != nil {
		return nil, err
	}
	return &settings, err
}

func condSetHWMForSource(meta *dataSource, ts int64) {
	if ts > meta.HWM {
		meta.HWM = ts
	}
}

func getString(mm map[string]interface{}, name string) (ret string) {
	byt, ok := mm[name].([]byte)
	if ok {
		ret = string(byt)
	}
	return
}

func getInt64(mm map[string]interface{}, name string) (ret int64) {
	byt, ok := mm[name]
	if ok {
		ret = byt.(int64)
	}
	return
}
