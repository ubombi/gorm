package reconnect

import (
	"time"

	"github.com/jinzhu/gorm"
)

var _ gorm.PluginInterface = &Reconnect{}

// Reconnect GORM reconnect plugin
type Reconnect struct {
	Config *Config
}

// Config reconnect config
type Config struct {
	Attempts uint
	Interval time.Duration
}

// New initialize GORM reconnect DB
func New(config *Config) *Reconnect {
	return &Reconnect{
		Config: config,
	}
}

// Apply apply reconnect to GORM DB instance
func (reconnect *Reconnect) Apply(db *gorm.DB) {
	db.Callback().Create().After("gorm:commit_or_rollback_transaction").Register("gorm:plugins:reconnect", performReconnect)
	db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("gorm:plugins:reconnect", performReconnect)
	db.Callback().Delete().After("gorm:commit_or_rollback_transaction").Register("gorm:plugins:reconnect", performReconnect)
	db.Callback().Query().After("gorm:query").Register("gorm:plugins:reconnect", performReconnect)
	db.Callback().RowQuery().After("gorm:row_query").Register("gorm:plugins:reconnect", performReconnect)
}

//performReconnect the callback used to peform some reconnect attempts in case of disconnect
func performReconnect(scope *Scope) {
	if scope.HasError() {

		scope.db.reconnectGuard.Add(1)
		defer scope.db.reconnectGuard.Done()

		err := scope.db.Error

		if scope.db.dialect.IsDisconnectError(err) {
			for i := 0; i < reconnectAttempts; i++ {
				newDb, openErr := Open(scope.db.dialectName, scope.db.dialectArgs...)
				if openErr == nil {
					oldDb := scope.db
					if oldDb.parent != oldDb {
						//In case of cloned db try to fix parents
						//It is thread safe as we share mutex between instances
						fixParentDbs(oldDb, newDb)
					}
					*scope.db = *newDb
					break
				} else {
					//wait for interval and try to reconnect again
					<-time.After(reconnectInterval)
				}
			}
		}
	}
}

func fixParentDbs(current, newDb *DB) {
	iterator := current
	parent := current.parent

	for {
		oldParent := parent
		*parent = *newDb
		parent = oldParent.parent
		iterator = oldParent
		if iterator == parent {
			break
		}
	}
}
