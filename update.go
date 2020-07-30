package sqlh

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// Executor is an interface with just *sql.DB.Exec behaviour.
type Executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// Update runs an SQL UPDATE query. It takes a database, target table,
// update value, and where clause with arguments.
//
//   type row struct{
//       Id int `sql:"id"`
//       Name string `sql:"name"`
//   }
//   res, err := Update(db, "X", row{Name: "updated"}, "id = ?", 1)
//
// Zero-values in the value struct are ignored.
func Update(db Executor, table string, value interface{}, where string, args ...interface{}) (sql.Result, error) {
	u, err := update(table, value, where, args...)
	if err != nil {
		return nil, err
	}
	return db.Exec(u.statement, u.args...)
}

type preUpdate struct {
	statement string
	args      []interface{}
}

func update(table string, value interface{}, where string, args ...interface{}) (*preUpdate, error) {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("update was not a struct: %v", v.Type())
	}
	var set []string
	var vals []interface{}

	var recurseFields func(t reflect.Type, index []int)
	recurseFields = func(t reflect.Type, index []int) {
		valIndex := len(args) + 1
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Anonymous {
				recurseFields(field.Type, append(index, field.Index...))
				continue
			}
			tag, ok := field.Tag.Lookup("sql")
			if !ok {
				continue // Ignore untagged
			}
			name, ignore := parseTag(tag, "update")
			if ignore {
				continue // Explicitly ignored
			}
			value := v.FieldByIndex(append(index, field.Index...))
			if value.IsZero() {
				continue // Ignore zero-value
			}
			set = append(set, name+fmt.Sprintf(" = $%d", valIndex))
			valIndex = valIndex + 1
			vals = append(vals, value.Interface())
		}
	}
	recurseFields(v.Type(), []int{})

	if len(set) < 1 {
		return nil, fmt.Errorf("no fields to update")
	}

	setStmt := strings.Join(set, ", ")
	stmt := fmt.Sprintf("UPDATE %s SET %s WHERE %s", table, setStmt, where)

	return &preUpdate{
		statement: stmt,
		args:      append(args, vals...),
	}, nil
}
