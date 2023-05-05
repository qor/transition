package transition

import (
	"fmt"
	gormv2 "gorm.io/gorm"
	"gorm.io/gorm/schema"
	"reflect"
	"strings"
	"sync"

	"github.com/qor/admin"
	"github.com/qor/audited"
	"github.com/qor/qor/resource"
	"github.com/qor/roles"
)

// StateChangeLog a model that used to keep state change logs
type StateChangeLog struct {
	gormv2.Model
	ReferTable string
	ReferID    string
	From       string
	To         string
	Note       string `sql:"size:1024"`
	audited.AuditedModel
}

var tableColumn = &sync.Map{}

func getStructFieldValueByName(myStruct interface{}, columnName string) interface{} {
	structValue := reflect.ValueOf(myStruct)
	fieldValue := structValue.FieldByName(columnName)
	return fieldValue.Interface()
}

// GenerateReferenceKey generate reference key used for change log
func GenerateReferenceKey(model interface{}, db *gormv2.DB) (string, error) {
	modelType := reflect.ValueOf(model)
	for modelType.Kind() == reflect.Slice || modelType.Kind() == reflect.Array || modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	if modelType.Kind() != reflect.Struct {
		return "", fmt.Errorf("modelType.Kind() != reflect.Struct,%d", modelType.Kind())
	}
	ss, err := schema.Parse(model, tableColumn, db.NamingStrategy)
	if err != nil {
		return "", err
	}
	var primaryValues []string
	for _, field := range ss.Fields {
		if field.PrimaryKey {
			primaryValues = append(primaryValues, fmt.Sprintf("%v",
				modelType.FieldByName(field.Name).Interface()))
		}
	}
	resStr := strings.Join(primaryValues, "::")
	fmt.Println(resStr)
	return resStr, nil
}

// GenTableName table name
func GenTableName(model interface{}, db *gormv2.DB) (string, error) {
	ss, err := schema.Parse(model, tableColumn, db.NamingStrategy)
	if err != nil {
		return "", err
	}
	return ss.Table, nil
}

// GetStateChangeLogs get state change logs
func GetStateChangeLogs(model interface{}, db *gormv2.DB) ([]StateChangeLog, error) {
	var (
		changelogs []StateChangeLog
	)
	tableName, err := GenTableName(model, db)
	if err != nil {
		return nil, err
	}
	key, err := GenerateReferenceKey(model, db)
	if err != nil {
		return nil, err
	}
	return changelogs, db.Where("refer_table = ? AND refer_id = ?", tableName, key).Find(&changelogs).Error
}

// GetLastStateChange gets last state change
func GetLastStateChange(model interface{}, db *gormv2.DB) (*StateChangeLog, error) {
	var (
		changelog StateChangeLog
	)
	tableName, err := GenTableName(model, db)
	if err != nil {
		return nil, err
	}
	key, err := GenerateReferenceKey(model, db)
	if err != nil {
		return nil, err
	}
	db.Where("refer_table = ? AND refer_id = ?", tableName, key).Last(&changelog)
	if changelog.To == "" {
		return nil, nil
	}
	return &changelog, nil
}

// ConfigureQorResource used to configure transition for qor admin
func (stageChangeLog *StateChangeLog) ConfigureQorResource(res resource.Resourcer) {
	if res, ok := res.(*admin.Resource); ok {
		if res.Permission == nil {
			res.Permission = roles.Deny(roles.Update, roles.Anyone).Deny(roles.Create, roles.Anyone)
		} else {
			res.Permission = res.Permission.Deny(roles.Update, roles.Anyone).Deny(roles.Create, roles.Anyone)
		}
	}
}
