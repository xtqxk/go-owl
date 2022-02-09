package owl

import (
	"reflect"
	"strconv"
)

func SetValue(fv reflect.Value, val string) (ret reflect.Value, err error) {
	switch fv.Type().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intval, serr := strconv.ParseInt(val, 10, 64)
		if serr != nil {
			err = serr
			return
		}
		fv.SetInt(intval)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, uerr := strconv.ParseUint(val, 10, 64)
		if uerr != nil {
			err = uerr
			return
		}
		fv.SetUint(uintVal)
	case reflect.Float64, reflect.Float32:
		fltval, serr := strconv.ParseFloat(val, 64)
		if serr != nil {
			err = serr
			return
		}
		fv.SetFloat(fltval)
	case reflect.Bool:
		bval, berr := strconv.ParseBool(val)
		if err != nil {
			err = berr
			return
		}
		fv.SetBool(bval)
	case reflect.String:
		fv.SetString(val)
	case reflect.Ptr:
		fl, rerr := SetValue(reflect.New(fv.Type().Elem()).Elem(), val)
		if rerr != nil {
			err = rerr
			return
		}
		fv.Set(fl.Addr())
	}
	return fv, nil
}
