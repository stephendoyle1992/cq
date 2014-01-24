package types

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type CypherType uint8

var (
	ErrScanOnNil = errors.New("cq: scan value is null")
)

// supported types
const (
	CypherNull             CypherType = iota
	CypherBoolean          CypherType = iota
	CypherString           CypherType = iota
	CypherInt64            CypherType = iota
	CypherInt              CypherType = iota
	CypherFloat64          CypherType = iota
	CypherArrayInt         CypherType = iota
	CypherArrayInt64       CypherType = iota
	CypherArrayByte        CypherType = iota
	CypherArrayFloat64     CypherType = iota
	CypherArrayString      CypherType = iota
	CypherArrayCypherValue CypherType = iota
	CypherMapStringString  CypherType = iota
	CypherNode             CypherType = iota
	CypherRelationship     CypherType = iota
	CypherPath             CypherType = iota
	CypherValueType        CypherType = iota
)

func (v *CypherValue) Scan(value interface{}) error {
	fmt.Println("attempting to Scan:", value)
	if v == nil {
		return ErrScanOnNil
	}
	if value == nil {
		v.Val = nil
		v.Type = CypherNull
		return nil
	}

	switch value.(type) {
	case bool:
		v.Type = CypherBoolean
		v.Val = value
		return nil
	case string:
		v.Type = CypherString
		v.Val = value
		return nil
	case int:
		if value.(int) > ((1 << 31) - 1) {
			v.Type = CypherInt64
			v.Val = int64(value.(int))
			return nil
		}
		v.Type = CypherInt
		v.Val = value
		return nil
	}

	err := json.Unmarshal(value.([]byte), &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case CypherArrayInt:
		var ai ArrayInt
		err = json.Unmarshal(value.([]byte), &ai.Val)
		v.Val = ai.Val
		return err
	}
	return err
}

type CypherValue struct {
	Type CypherType
	Val  interface{}
}

func (cv *CypherValue) Value() (driver.Value, error) {
	fmt.Println(cv, "CV: Value()")
	fmt.Println(cv.Val)
	b, err := json.Marshal(cv)
	return b, err
}

func (c *CypherValue) UnmarshalJSON(b []byte) error {
	fmt.Println("attempting to unmarshal: ", string(b))
	if len(b) > 0 && b[0] == '{' {
		start := bytes.Index(b, []byte(":"))
		end := bytes.Index(b, []byte(","))
		if start > 0 && end > 0 {
			t, err := strconv.Atoi(string(b[start:end]))
			if err != nil {
				return err
			}
			c.Type = CypherType(t)
			switch c.Type {
			case CypherArrayInt:
				fmt.Println("cypher array int: ", string(b[10:]))
				var ai = []int{}
				err := json.Unmarshal(b[10:len(b)-1], &ai)
				if err != nil {
					return err
				}
				c.Val = ai
				return nil
			}
		}
	}
	fmt.Println("got too far...")
	var err error
	str := string(b)
	switch str {
	case "null":
		c.Val = nil
		c.Type = CypherNull
		return nil
	case "true":
		c.Val = true
		c.Type = CypherBoolean
		return nil
	case "false":
		c.Val = false
		c.Type = CypherBoolean
		return nil
	}
	if len(b) > 0 {
		switch b[0] {
		case byte('"'):
			c.Val = strings.Trim(str, "\"")
			c.Type = CypherString
			return nil
		case byte('{'):
			c.Val = b
			c.Type = CypherValueType
			return nil
		case byte('['):
			c.Val = b
			c.Type = CypherArrayInt
			return nil
		}
	}
	c.Val, err = strconv.Atoi(str)
	if err == nil {
		c.Type = CypherInt
		return nil
	}
	c.Val, err = strconv.ParseInt(str, 10, 64)
	if err == nil {
		c.Type = CypherInt64
		return nil
	}
	c.Val, err = strconv.ParseFloat(str, 64)
	if err == nil {
		c.Type = CypherFloat64
		return nil
	}
	c.Val = b
	c.Type = CypherValueType
	//json.Unmarshal(b, &c.Val)
	return nil
}

func (cv CypherValue) ConvertValue(v interface{}) (driver.Value, error) {
	fmt.Println("attempting to convert:", v)
	if driver.IsValue(v) {
		fmt.Println("IsValue:", v)
		return v, nil
	}

	if svi, ok := v.(driver.Valuer); ok {
		fmt.Println("we have a valuer:", v)
		sv, err := svi.Value()
		if err != nil {
			return nil, err
		}
		if !driver.IsValue(sv) {
			return nil, fmt.Errorf("non-Value type %T returned from Value", sv)
		}
		return sv, nil
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		fmt.Println("converting slice")
		b := CypherValue{}
		switch v.(type) {
		case []int:
			fmt.Println("converting []int")
			b.Type = CypherArrayInt
			b.Val = v
		}
		return b.Value()
	case reflect.Map:
		b, err := json.Marshal(v)
		return b, err
	case reflect.Ptr:
		// indirect pointers
		if rv.IsNil() {
			return nil, nil
		} else {
			return CypherValue{}.ConvertValue(rv.Elem().Interface())
		}
	}
	return nil, fmt.Errorf("unsupported type %T, a %s", v, rv.Kind())
}
