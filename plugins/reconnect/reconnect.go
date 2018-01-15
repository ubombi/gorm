package reconnect

import (
	"database/sql/driver"
	"reflect"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
)

var _ gorm.PluginInterface = &Reconnect{}

// Reconnect GORM reconnect plugin
type Reconnect struct {
	Config *Config
	mutex  *sync.Mutex
}

// Config reconnect config
type Config struct {
	Attempts       int
	Interval       time.Duration
	BadConnChecker func(errors []error) bool
}

// New initialize GORM reconnect DB
func New(config *Config) *Reconnect {
	if config == nil {
		config = &Config{}
	}

	if config.BadConnChecker == nil {
		config.BadConnChecker = func(errors []error) bool {
			for _, err := range errors {
				if err == driver.ErrBadConn {
					return true
				}
			}
			return false
		}
	}

	if config.Attempts == 0 {
		config.Attempts = 5
	}

	if config.Interval == 0 {
		config.Interval = 5 * time.Second
	}

	return &Reconnect{
		mutex:  &sync.Mutex{},
		Config: config,
	}
}

// Apply apply reconnect to GORM DB instance
func (reconnect *Reconnect) Apply(db *gorm.DB) {
	db.Callback().Create().After("gorm:commit_or_rollback_transaction").Register("gorm:plugins:reconnect", reconnect.generateCallback("creates"))
	db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("gorm:plugins:reconnect", reconnect.generateCallback("updates"))
	db.Callback().Delete().After("gorm:commit_or_rollback_transaction").Register("gorm:plugins:reconnect", reconnect.generateCallback("deletes"))
	db.Callback().Query().After("gorm:query").Register("gorm:plugins:reconnect", reconnect.generateCallback("queries"))
	db.Callback().RowQuery().After("gorm:row_query").Register("gorm:plugins:reconnect", reconnect.generateCallback("rowQueries"))
}

//performReconnect the callback used to peform some reconnect attempts in case of disconnect
func (reconnect *Reconnect) generateCallback(callbackType string) func(*gorm.Scope) {
	return func(scope *gorm.Scope) {
		if scope.HasError() {
			if db := scope.DB(); reconnect.Config.BadConnChecker(db.GetErrors()) {
				reconnect.mutex.Lock()

				connected := db.DB().Ping() == nil

				if !connected {
					for i := 0; i < reconnect.Config.Attempts; i++ {
						if err := reconnect.reconnectDB(db); err == nil {
							connected = true
							break
						}
						time.Sleep(reconnect.Config.Interval)
					}
				}

				reconnect.mutex.Unlock()

				if connected {
					// TODO reexec current command
				}
			}
		}
	}
}

func (reconnect *Reconnect) reconnectDB(db *gorm.DB) error {
	var (
		sqlDB      = db.DB()
		dsn        = reflect.Indirect(reflect.ValueOf(sqlDB)).FieldByName("dsn").String()
		newDB, err = gorm.Open(db.Dialect().GetName(), dsn)
	)

	if err == nil {
		*sqlDB = *newDB.DB()
	}

	return err
}
