package psg

import (
	"HW1_http/models/dto"
	"HW1_http/pkg"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"html/template"
	"reflect"
	"strconv"
	"strings"
)

func (p *Psg) RecordSave(rd dto.Record) error {
	eW := pkg.NewEWrapper("(p *Psg) RecordSave()")

	err := p.PhoneExists(rd.Phone)
	if err != nil {
		return eW.WrapError(err, "p.PhoneExists(rd.Phone)")
	}

	sqlCommand := `INSERT INTO address_book (name, last_name, middle_name, address, phone) VALUES ($1, $2, $3, $4, $5)`
	_, err = p.conn.Exec(context.Background(), sqlCommand, rd.Name, rd.LastName, rd.MiddleName, rd.Address, rd.Phone)
	if err != nil {
		return eW.WrapError(err, "p.db.Exec()")
	}
	return nil
}

func (p *Psg) RecordsGet(record dto.Record) (result []dto.Record, err error) {
	eW := pkg.NewEWrapper("(p *Psg) RecordsGet()")

	sqlCommand, values, err := p.SelectRecord(record)
	if err != nil {
		return result, eW.WrapError(err, "p.SelectRecord(record)")
	}

	rows, err := p.conn.Query(context.Background(), sqlCommand, values...)
	if err != nil {
		return result, eW.WrapError(err, "p.db.Query()")
	}
	defer rows.Close()
	for rows.Next() {
		var r dto.Record
		if err := rows.Scan(&r.ID, &r.Name, &r.LastName, &r.MiddleName, &r.Address, &r.Phone); err != nil {
			return result, eW.WrapError(err, "rows.Scan(&r.ID, &r.Name, &r.LastName, &r.MiddleName, &r.Address, &r.Phone)")
		}
		result = append(result, r)
	}

	if err := rows.Err(); err != nil {
		return result, eW.WrapError(err, "rows.Err()")
	}

	return result, nil
}

func (p *Psg) RecordUpdate(record dto.Record) (err error) {
	eW := pkg.NewEWrapper("(p *Psg) RecordUpdate()")

	err = p.PhoneExists(record.Phone)
	if err == nil {
		return eW.WrapError(errors.New(""), "Phone does not exist")
	}
	err = nil

	fields := []string{}
	values := []interface{}{}
	index := 1

	if record.Name != "" {
		fields = append(fields, fmt.Sprintf("name=$%d", index))
		values = append(values, record.Name)
		index++
	}
	if record.LastName != "" {
		fields = append(fields, fmt.Sprintf("last_name=$%d", index))
		values = append(values, record.LastName)
		index++
	}
	if record.MiddleName != "" {
		fields = append(fields, fmt.Sprintf("middle_name=$%d", index))
		values = append(values, record.MiddleName)
		index++
	}
	if record.Address != "" {
		fields = append(fields, fmt.Sprintf("address=$%d", index))
		values = append(values, record.Address)
		index++
	}

	values = append(values, record.Phone)
	sqlCommand := fmt.Sprintf(`UPDATE address_book SET %s WHERE phone=$%d`, strings.Join(fields, ", "), index)

	_, err = p.conn.Exec(context.Background(), sqlCommand, values...)
	if err != nil {
		return eW.WrapError(err, "p.db.Exec()")
	}
	return nil
}

func (p *Psg) RecordDeleteByPhone(phone string) (err error) {
	eW := pkg.NewEWrapper("(p *Psg) RecordDeleteByPhone()")

	err = p.PhoneExists(phone)
	if err == nil {
		return eW.WrapError(errors.New(""), "Phone does not exist")
	}

	sqlCommand := `DELETE FROM address_book WHERE phone=$1`
	_, err = p.conn.Exec(context.Background(), sqlCommand, phone)
	if err != nil {
		return eW.WrapError(err, "p.db.Exec()")
	}
	return nil
}

func (p *Psg) SelectRecord(r dto.Record) (res_query string, values []any, err error) {
	sqlFields, values, err := structToFieldsValues(r, "sql.field")
	if err != nil {
		return
	}

	var conds []dto.Cond

	for i := range sqlFields {
		if i == 0 {
			conds = append(conds, dto.Cond{
				Lop:    "",
				PgxInd: "$" + strconv.Itoa(i+1),
				Field:  sqlFields[i],
				Value:  values[i],
			})
			continue
		}
		conds = append(conds, dto.Cond{
			Lop:    "AND",
			PgxInd: "$" + strconv.Itoa(i+1),
			Field:  sqlFields[i],
			Value:  values[i],
		})
	}

	query := `
	SELECT 
		id, name, last_name, middle_name, address, phone
	FROM
	    address_book
	WHERE
		{{range .}} {{.Lop}} {{.Field}} = {{.PgxInd}}{{end}}
;
`
	tmpl, err := template.New("").Parse(query)
	if err != nil {
		return
	}

	var sb strings.Builder
	err = tmpl.Execute(&sb, conds)
	if err != nil {
		return
	}
	res_query = sb.String()
	return
}

func structToFieldsValues(s any, tag string) (sqlFields []string, values []any, err error) {
	rv := reflect.ValueOf(s)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, nil, errors.New("s must be a struct")
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		tg := strings.TrimSpace(field.Tag.Get(tag))
		if tg == "" || tg == "-" {
			continue
		}
		tgs := strings.Split(tg, ",")
		tg = tgs[0]

		fv := rv.Field(i)
		isZero := false
		switch fv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			isZero = fv.Int() == 0
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			isZero = fv.Uint() == 0
		case reflect.Float32, reflect.Float64:
			isZero = fv.Float() == 0
		case reflect.Complex64, reflect.Complex128:
			isZero = fv.Complex() == complex(0, 0)
		case reflect.Bool:
			isZero = !fv.Bool()
		case reflect.String:
			isZero = fv.String() == ""
		case reflect.Array, reflect.Slice:
			isZero = fv.Len() == 0
		}

		if isZero {
			continue
		}

		sqlFields = append(sqlFields, tg)
		values = append(values, fv.Interface())
	}

	return
}

func (p *Psg) PhoneExists(phone string) error {
	eW := pkg.NewEWrapper("(p *Psg) PhoneExists()")
	sqlCommand := `SELECT phone FROM address_book WHERE phone = $1`
	row := p.conn.QueryRow(context.Background(), sqlCommand, phone)
	var existingPhone string
	err := row.Scan(&existingPhone)
	if existingPhone == phone {
		return errors.New("phone number already in use")
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return eW.WrapError(err, "row.Scan(&existingPhone)")
	}
	return eW.WrapError(err, "phone number already in use")
}
